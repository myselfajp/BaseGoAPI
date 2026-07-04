package service

import (
	"time"

	"github.com/myselfajp/BaseGoAPI/internal/apperror"
	"github.com/myselfajp/BaseGoAPI/internal/config"
	"github.com/myselfajp/BaseGoAPI/internal/core/security"
	"github.com/myselfajp/BaseGoAPI/internal/repository"
)

// EmailVerificationService issues and validates email-verification tokens.
type EmailVerificationService struct {
	cfg         *config.Config
	tokenRepo   *repository.EmailVerificationTokenRepository
	userService *UserService
}

// NewEmailVerificationService builds the service.
func NewEmailVerificationService(
	cfg *config.Config,
	tokenRepo *repository.EmailVerificationTokenRepository,
	userService *UserService,
) *EmailVerificationService {
	return &EmailVerificationService{cfg: cfg, tokenRepo: tokenRepo, userService: userService}
}

// CreateVerificationToken generates and stores a new verification token for a
// user, returning the token value to be emailed.
func (s *EmailVerificationService) CreateVerificationToken(userID uint) (string, error) {
	token, err := security.GenerateURLSafeToken(32)
	if err != nil {
		return "", apperror.Internal("Failed to generate verification token.")
	}

	expiresAt := time.Now().UTC().Add(
		time.Duration(s.cfg.EmailVerificationTokenExpirationMinutes) * time.Minute,
	)
	if _, err := s.tokenRepo.CreateToken(userID, token, expiresAt); err != nil {
		return "", err
	}
	return token, nil
}

// VerifyToken validates a verification token, marks the user's email verified
// and returns the user id.
func (s *EmailVerificationService) VerifyToken(token string) (uint, error) {
	record, err := s.tokenRepo.GetByToken(token)
	if err != nil {
		return 0, err
	}
	if record == nil {
		return 0, apperror.BadRequest("Invalid verification token.")
	}
	if record.UsedAt != nil {
		return 0, apperror.BadRequest("This verification token has already been used.")
	}
	if record.ExpiresAt.Before(time.Now().UTC()) {
		return 0, apperror.BadRequest("Verification token has expired. Please request a new verification email.")
	}

	if err := s.tokenRepo.MarkAsUsed(record); err != nil {
		return 0, err
	}

	user, err := s.userService.VerifyEmail(record.UserID)
	if err != nil {
		return 0, err
	}

	// The user is verified now; drop any remaining tokens.
	if err := s.tokenRepo.PurgeTokensForUser(record.UserID); err != nil {
		return 0, err
	}
	return user.ID, nil
}
