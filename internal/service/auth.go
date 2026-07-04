package service

import (
	"strconv"
	"strings"
	"time"

	"github.com/myselfajp/BaseGoAPI/internal/apperror"
	"github.com/myselfajp/BaseGoAPI/internal/config"
	"github.com/myselfajp/BaseGoAPI/internal/core/email"
	"github.com/myselfajp/BaseGoAPI/internal/core/jwtutil"
	"github.com/myselfajp/BaseGoAPI/internal/core/security"
	"github.com/myselfajp/BaseGoAPI/internal/dto"
	"github.com/myselfajp/BaseGoAPI/internal/model"
	"github.com/myselfajp/BaseGoAPI/internal/repository"
)

// maxFailedLoginAttempts is the number of failed attempts (for non-admin
// accounts) that triggers an account lock.
const maxFailedLoginAttempts = 3

// AuthService implements authentication, including the OTP-based second factor.
type AuthService struct {
	cfg          *config.Config
	userRepo     *repository.UserRepository
	loginOTPRepo *repository.LoginOTPRepository
	emailSender  email.Sender
}

// NewAuthService builds an AuthService.
func NewAuthService(
	cfg *config.Config,
	userRepo *repository.UserRepository,
	loginOTPRepo *repository.LoginOTPRepository,
	emailSender email.Sender,
) *AuthService {
	return &AuthService{cfg: cfg, userRepo: userRepo, loginOTPRepo: loginOTPRepo, emailSender: emailSender}
}

// Login authenticates a user with a password and enforces the OTP second factor
// when required. It returns either a *dto.LoginResponse (fully authenticated)
// or a *dto.LoginChallengeResponse (OTP required).
func (s *AuthService) Login(email, password, otpCode, otpChallengeID string) (any, error) {
	user, err := s.userRepo.GetByEmail(strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, apperror.Unauthorized("Incorrect credentials")
	}

	if s.cfg.RequireEmailVerification && !user.IsEmailVerified {
		return nil, apperror.Forbidden(
			"Please verify your email address before logging in. Check your inbox for the verification email.",
		)
	}

	if !security.VerifyPassword(password, user.PasswordHash) {
		return nil, s.handleFailedPassword(user)
	}

	if !user.IsActive {
		return nil, apperror.Forbidden(
			"Your account has been locked. Please contact an administrator to unlock your account.",
		)
	}

	// Successful password: reset the failure counter for non-admin accounts.
	if !user.IsAdmin() {
		user.FailedLoginAttempts = 0
		user.LastFailedLogin = nil
		if err := s.userRepo.Save(user); err != nil {
			return nil, err
		}
	}

	requires2FA := s.cfg.ForceTwoFactorAuth || user.IsTwoFactorEnabled
	if requires2FA {
		if otpCode != "" {
			if otpChallengeID == "" {
				return nil, apperror.BadRequest("otp_challenge_id is required when submitting an OTP code.")
			}
			return s.completeLoginWithOTP(user, otpChallengeID, otpCode)
		}
		if otpChallengeID != "" {
			return nil, apperror.BadRequest("OTP code is required when submitting an otp_challenge_id.")
		}
		return s.initiateLoginWithOTP(user)
	}

	return s.buildLoginResponse(user)
}

// handleFailedPassword applies the lockout policy for a wrong password.
func (s *AuthService) handleFailedPassword(user *model.User) error {
	// Admins are never locked out by failed attempts.
	if user.IsAdmin() {
		return apperror.Unauthorized("Incorrect credentials")
	}

	now := time.Now().UTC()
	user.FailedLoginAttempts++
	user.LastFailedLogin = &now

	if user.FailedLoginAttempts >= maxFailedLoginAttempts {
		user.IsActive = false
		user.FailedLoginAttempts = 0
		if err := s.userRepo.Save(user); err != nil {
			return err
		}
		return apperror.Forbidden(
			"Your account has been locked due to multiple failed login attempts. Please contact an administrator to unlock your account.",
		)
	}

	if err := s.userRepo.Save(user); err != nil {
		return err
	}
	remaining := maxFailedLoginAttempts - user.FailedLoginAttempts
	return apperror.Unauthorized(
		"Incorrect credentials. " + strconv.Itoa(remaining) + " attempt(s) remaining before account lock.",
	)
}

// initiateLoginWithOTP generates and emails an OTP challenge.
func (s *AuthService) initiateLoginWithOTP(user *model.User) (any, error) {
	if err := s.loginOTPRepo.PurgePendingForUser(user.ID); err != nil {
		return nil, err
	}

	otpLength := max(1, s.cfg.OTPLength)
	otpCode, err := security.GenerateNumericOTP(otpLength)
	if err != nil {
		return nil, apperror.Internal("Failed to generate OTP.")
	}

	expiresMinutes := max(1, s.cfg.OTPExpirationMinutes)
	expiresAt := time.Now().UTC().Add(time.Duration(expiresMinutes) * time.Minute)

	codeHash, err := security.HashPassword(otpCode)
	if err != nil {
		return nil, apperror.Internal("Failed to process OTP.")
	}

	record, err := s.loginOTPRepo.CreateChallenge(user.ID, codeHash, expiresAt)
	if err != nil {
		return nil, err
	}

	content := BuildOTPEmail(s.cfg, user.FullName, otpCode, expiresMinutes)
	if err := s.emailSender.Send(user.Email, content.Subject, content.BodyText, content.BodyHTML); err != nil {
		// Roll back the challenge if delivery fails.
		_ = s.loginOTPRepo.PurgePendingForUser(user.ID)
		return nil, apperror.Internal("Failed to deliver OTP. Please try again later.")
	}

	return &dto.LoginChallengeResponse{
		Status:         "otp_required",
		ChallengeID:    record.ID,
		ExpiresIn:      int(time.Until(expiresAt).Seconds()),
		DeliveryMethod: "email",
		Destination:    maskEmail(user.Email),
		Message:        "OTP has been sent to your email address.",
	}, nil
}

// completeLoginWithOTP validates a submitted OTP and finishes login.
func (s *AuthService) completeLoginWithOTP(user *model.User, challengeID, otpCode string) (any, error) {
	record, err := s.loginOTPRepo.GetByID(challengeID)
	if err != nil {
		return nil, err
	}
	if record == nil || record.UserID != user.ID {
		return nil, apperror.BadRequest("Invalid OTP challenge. Please request a new code.")
	}

	if record.ConsumedAt != nil {
		return nil, apperror.BadRequest("OTP has already been used. Please request a new code.")
	}

	if record.ExpiresAt.Before(time.Now().UTC()) {
		_ = s.loginOTPRepo.PurgePendingForUser(user.ID)
		return nil, apperror.BadRequest("OTP has expired. Please request a new code.")
	}

	if !security.VerifyPassword(otpCode, record.CodeHash) {
		return nil, apperror.Unauthorized("Incorrect OTP code.")
	}

	if err := s.loginOTPRepo.Consume(record); err != nil {
		return nil, err
	}
	return s.buildLoginResponse(user)
}

// buildLoginResponse issues a JWT and assembles the login response.
func (s *AuthService) buildLoginResponse(user *model.User) (*dto.LoginResponse, error) {
	token, err := jwtutil.CreateAccessToken(s.cfg, user.Email, 0)
	if err != nil {
		return nil, apperror.Internal("Failed to issue access token.")
	}
	expiresIn := s.cfg.AccessTokenExpireMinutes
	if expiresIn <= 0 {
		expiresIn = 1440
	}
	return &dto.LoginResponse{
		Status:      "authenticated",
		AccessToken: token,
		TokenType:   "bearer",
		ExpiresIn:   expiresIn,
		User: dto.UserInfo{
			ID:                 user.ID,
			Email:              user.Email,
			FullName:           user.FullName,
			PhoneNumber:        user.PhoneNumber,
			Role:               user.Role,
			IsActive:           user.IsActive,
			IsEmailVerified:    user.IsEmailVerified,
			IsTwoFactorEnabled: user.IsTwoFactorEnabled,
			CreatedAt:          user.CreatedAt,
		},
	}, nil
}

// maskEmail obscures the local part of an email for display.
func maskEmail(email string) string {
	local, domain, found := strings.Cut(email, "@")
	if !found || local == "" || domain == "" {
		return email
	}
	var masked string
	if len(local) <= 2 {
		masked = local[:1] + "***"
	} else {
		masked = string(local[0]) + "***" + string(local[len(local)-1])
	}
	return masked + "@" + domain
}
