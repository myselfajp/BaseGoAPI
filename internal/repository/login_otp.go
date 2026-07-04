package repository

import (
	"time"

	"gorm.io/gorm"

	"github.com/myselfajp/BaseGoAPI/internal/model"
)

// LoginOTPRepository is the data-access layer for login OTP challenges.
type LoginOTPRepository struct {
	*BaseRepository[model.LoginOTP]
}

// NewLoginOTPRepository builds a LoginOTPRepository.
func NewLoginOTPRepository(db *gorm.DB) *LoginOTPRepository {
	return &LoginOTPRepository{BaseRepository: NewBaseRepository[model.LoginOTP](db, "LoginOTP")}
}

// CreateChallenge stores a new OTP challenge for a user.
func (r *LoginOTPRepository) CreateChallenge(userID uint, codeHash string, expiresAt time.Time) (*model.LoginOTP, error) {
	otp := &model.LoginOTP{UserID: userID, CodeHash: codeHash, ExpiresAt: expiresAt}
	if err := r.Create(otp); err != nil {
		return nil, err
	}
	return otp, nil
}

// GetByID returns the OTP challenge with the given id, or (nil, nil) when none.
func (r *LoginOTPRepository) GetByID(id string) (*model.LoginOTP, error) {
	return r.Get(id)
}

// Consume marks an OTP challenge as used.
func (r *LoginOTPRepository) Consume(otp *model.LoginOTP) error {
	now := time.Now().UTC()
	otp.ConsumedAt = &now
	return r.Save(otp)
}

// PurgePendingForUser deletes all unconsumed OTP challenges for a user.
func (r *LoginOTPRepository) PurgePendingForUser(userID uint) error {
	return r.DB().
		Where("user_id = ? AND consumed_at IS NULL", userID).
		Delete(&model.LoginOTP{}).Error
}
