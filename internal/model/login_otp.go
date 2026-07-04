package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LoginOTP is a one-time password challenge issued during two-factor login.
type LoginOTP struct {
	ID         string     `gorm:"size:36;primaryKey" json:"id"`
	UserID     uint       `gorm:"not null;index" json:"user_id"`
	CodeHash   string     `gorm:"size:255;not null" json:"-"`
	ExpiresAt  time.Time  `gorm:"not null" json:"expires_at"`
	ConsumedAt *time.Time `json:"consumed_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

// TableName pins the table name.
func (LoginOTP) TableName() string { return "login_otps" }

// BeforeCreate assigns a UUID primary key when one has not been set.
func (o *LoginOTP) BeforeCreate(*gorm.DB) error {
	if o.ID == "" {
		o.ID = uuid.NewString()
	}
	return nil
}
