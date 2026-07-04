package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PasswordResetToken is a single-use token emailed to a user to authorise a
// password reset.
type PasswordResetToken struct {
	ID        string     `gorm:"size:36;primaryKey" json:"id"`
	UserID    uint       `gorm:"not null;index" json:"user_id"`
	Token     string     `gorm:"size:255;uniqueIndex;not null" json:"token"`
	ExpiresAt time.Time  `gorm:"not null" json:"expires_at"`
	UsedAt    *time.Time `json:"used_at"`
	CreatedAt time.Time  `json:"created_at"`
}

// TableName pins the table name.
func (PasswordResetToken) TableName() string { return "password_reset_tokens" }

// BeforeCreate assigns a UUID primary key when one has not been set.
func (t *PasswordResetToken) BeforeCreate(*gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	return nil
}
