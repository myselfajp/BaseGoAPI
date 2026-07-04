package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/myselfajp/BaseGoAPI/internal/model"
)

// EmailVerificationTokenRepository is the data-access layer for email
// verification tokens.
type EmailVerificationTokenRepository struct {
	*BaseRepository[model.EmailVerificationToken]
}

// NewEmailVerificationTokenRepository builds the repository.
func NewEmailVerificationTokenRepository(db *gorm.DB) *EmailVerificationTokenRepository {
	return &EmailVerificationTokenRepository{
		BaseRepository: NewBaseRepository[model.EmailVerificationToken](db, "EmailVerificationToken"),
	}
}

// CreateToken stores a new verification token.
func (r *EmailVerificationTokenRepository) CreateToken(userID uint, token string, expiresAt time.Time) (*model.EmailVerificationToken, error) {
	record := &model.EmailVerificationToken{UserID: userID, Token: token, ExpiresAt: expiresAt}
	if err := r.Create(record); err != nil {
		return nil, err
	}
	return record, nil
}

// GetByToken returns the record for a token value, or (nil, nil) when none.
func (r *EmailVerificationTokenRepository) GetByToken(token string) (*model.EmailVerificationToken, error) {
	var record model.EmailVerificationToken
	err := r.DB().Where("token = ?", token).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// MarkAsUsed stamps the token as used.
func (r *EmailVerificationTokenRepository) MarkAsUsed(record *model.EmailVerificationToken) error {
	now := time.Now().UTC()
	record.UsedAt = &now
	return r.Save(record)
}

// PurgeExpiredTokens deletes every expired verification token.
func (r *EmailVerificationTokenRepository) PurgeExpiredTokens() error {
	return r.DB().
		Where("expires_at < ?", time.Now().UTC()).
		Delete(&model.EmailVerificationToken{}).Error
}

// PurgeTokensForUser deletes every verification token for a user.
func (r *EmailVerificationTokenRepository) PurgeTokensForUser(userID uint) error {
	return r.DB().
		Where("user_id = ?", userID).
		Delete(&model.EmailVerificationToken{}).Error
}
