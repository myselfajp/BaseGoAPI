package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/myselfajp/BaseGoAPI/internal/model"
)

// PasswordResetTokenRepository is the data-access layer for password reset
// tokens.
type PasswordResetTokenRepository struct {
	*BaseRepository[model.PasswordResetToken]
}

// NewPasswordResetTokenRepository builds the repository.
func NewPasswordResetTokenRepository(db *gorm.DB) *PasswordResetTokenRepository {
	return &PasswordResetTokenRepository{
		BaseRepository: NewBaseRepository[model.PasswordResetToken](db, "PasswordResetToken"),
	}
}

// CreateToken stores a new password reset token.
func (r *PasswordResetTokenRepository) CreateToken(userID uint, token string, expiresAt time.Time) (*model.PasswordResetToken, error) {
	record := &model.PasswordResetToken{UserID: userID, Token: token, ExpiresAt: expiresAt}
	if err := r.Create(record); err != nil {
		return nil, err
	}
	return record, nil
}

// GetByToken returns the record for a token value, or (nil, nil) when none.
func (r *PasswordResetTokenRepository) GetByToken(token string) (*model.PasswordResetToken, error) {
	var record model.PasswordResetToken
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
func (r *PasswordResetTokenRepository) MarkAsUsed(record *model.PasswordResetToken) error {
	now := time.Now().UTC()
	record.UsedAt = &now
	return r.Save(record)
}

// PurgeExpiredTokens deletes every expired reset token.
func (r *PasswordResetTokenRepository) PurgeExpiredTokens() error {
	return r.DB().
		Where("expires_at < ?", time.Now().UTC()).
		Delete(&model.PasswordResetToken{}).Error
}

// PurgeTokensForUser deletes every reset token for a user.
func (r *PasswordResetTokenRepository) PurgeTokensForUser(userID uint) error {
	return r.DB().
		Where("user_id = ?", userID).
		Delete(&model.PasswordResetToken{}).Error
}
