package service

import (
	"strings"
	"time"

	"github.com/myselfajp/BaseGoAPI/internal/apperror"
	"github.com/myselfajp/BaseGoAPI/internal/config"
	"github.com/myselfajp/BaseGoAPI/internal/core/security"
	"github.com/myselfajp/BaseGoAPI/internal/repository"
)

// PasswordResetService issues and consumes password-reset tokens.
type PasswordResetService struct {
	cfg       *config.Config
	tokenRepo *repository.PasswordResetTokenRepository
	userRepo  *repository.UserRepository
}

// NewPasswordResetService builds the service.
func NewPasswordResetService(
	cfg *config.Config,
	tokenRepo *repository.PasswordResetTokenRepository,
	userRepo *repository.UserRepository,
) *PasswordResetService {
	return &PasswordResetService{cfg: cfg, tokenRepo: tokenRepo, userRepo: userRepo}
}

// CreateResetToken generates a reset token for the account with the given email.
// It returns an empty token (and no error) when no such account exists, so the
// caller can present an identical response regardless of existence and prevent
// email enumeration.
func (s *PasswordResetService) CreateResetToken(email string) (string, error) {
	user, err := s.userRepo.GetByEmail(strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", nil
	}

	token, err := security.GenerateURLSafeToken(32)
	if err != nil {
		return "", apperror.Internal("Failed to generate reset token.")
	}

	expiresAt := time.Now().UTC().Add(
		time.Duration(s.cfg.PasswordResetTokenExpirationMinutes) * time.Minute,
	)

	if err := s.tokenRepo.PurgeTokensForUser(user.ID); err != nil {
		return "", err
	}
	if _, err := s.tokenRepo.CreateToken(user.ID, token, expiresAt); err != nil {
		return "", err
	}
	return token, nil
}

// ResetPassword validates a reset token and sets a new password for the user.
func (s *PasswordResetService) ResetPassword(token, newPassword string) (uint, error) {
	record, err := s.tokenRepo.GetByToken(token)
	if err != nil {
		return 0, err
	}
	if record == nil {
		return 0, apperror.BadRequest("Invalid reset token.")
	}
	if record.UsedAt != nil {
		return 0, apperror.BadRequest("This reset token has already been used.")
	}
	if record.ExpiresAt.Before(time.Now().UTC()) {
		return 0, apperror.BadRequest("Reset token has expired. Please request a new password reset.")
	}

	user, err := s.userRepo.Get(record.UserID)
	if err != nil {
		return 0, err
	}
	if user == nil {
		return 0, apperror.NotFound("User not found.")
	}

	hashed, err := security.HashPassword(newPassword)
	if err != nil {
		return 0, apperror.Internal("Failed to hash password")
	}
	user.PasswordHash = hashed
	if err := s.userRepo.Save(user); err != nil {
		return 0, err
	}

	if err := s.tokenRepo.MarkAsUsed(record); err != nil {
		return 0, err
	}
	if err := s.tokenRepo.PurgeTokensForUser(user.ID); err != nil {
		return 0, err
	}
	return user.ID, nil
}
