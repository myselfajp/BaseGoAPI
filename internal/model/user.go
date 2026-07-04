// Package model contains the GORM data models. It is the Go equivalent of
// app/model/*.py.
package model

import "time"

// Role constants. The base template ships with two generic roles; extend this
// set for your own application.
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// User is the account model.
type User struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	Email        string `gorm:"size:255;uniqueIndex;not null" json:"email"`
	PasswordHash string `gorm:"size:255;not null" json:"-"`
	Role         string `gorm:"size:50;not null" json:"role"`
	// No gorm default tag here: with `default:true` GORM would omit a false
	// value from INSERTs (false is Go's zero value) and the database default
	// would silently flip the user back to active.
	IsActive            bool       `gorm:"not null" json:"is_active"`
	FullName            string     `gorm:"size:255;not null;default:''" json:"full_name"`
	PhoneNumber         string     `gorm:"size:20;not null;default:''" json:"phone_number"`
	IsEmailVerified     bool       `gorm:"not null;default:false" json:"is_email_verified"`
	IsTwoFactorEnabled  bool       `gorm:"not null;default:false" json:"is_two_factor_enabled"`
	FailedLoginAttempts int        `gorm:"not null;default:0" json:"failed_login_attempts"`
	LastFailedLogin     *time.Time `json:"last_failed_login"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// TableName pins the table name so it is independent of GORM's pluralizer.
func (User) TableName() string { return "users" }

// IsAdmin reports whether the user has the admin role.
func (u *User) IsAdmin() bool { return u.Role == RoleAdmin }
