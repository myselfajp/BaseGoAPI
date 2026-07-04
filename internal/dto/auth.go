// Package dto holds the request and response payloads for the HTTP layer. It is
// the Go equivalent of app/schema/*.py.
package dto

import "time"

// --- Requests ---

// LoginInput is the body of POST /v1/auth/login.
type LoginInput struct {
	Email          string `json:"email" binding:"required,email"`
	Password       string `json:"password" binding:"required"`
	OTPCode        string `json:"otp_code"`
	OTPChallengeID string `json:"otp_challenge_id"`
}

// RegisterInput is the body of POST /v1/auth/register.
type RegisterInput struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required"`
	FullName    string `json:"full_name" binding:"required"`
	PhoneNumber string `json:"phone_number" binding:"required"`
}

// ForgotPasswordInput is the body of POST /v1/auth/forgot-password.
type ForgotPasswordInput struct {
	Email string `json:"email" binding:"required,email"`
}

// ResetPasswordInput is the body of POST /v1/auth/reset-password.
type ResetPasswordInput struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// --- Responses ---

// UserInfo is the user representation embedded in the login response.
type UserInfo struct {
	ID                 uint      `json:"id"`
	Email              string    `json:"email"`
	FullName           string    `json:"full_name"`
	PhoneNumber        string    `json:"phone_number"`
	Role               string    `json:"role"`
	IsActive           bool      `json:"is_active"`
	IsEmailVerified    bool      `json:"is_email_verified"`
	IsTwoFactorEnabled bool      `json:"is_two_factor_enabled"`
	CreatedAt          time.Time `json:"created_at"`
}

// LoginResponse is returned on a successful, fully-authenticated login.
type LoginResponse struct {
	Status      string   `json:"status"` // always "authenticated"
	AccessToken string   `json:"access_token"`
	TokenType   string   `json:"token_type"`
	ExpiresIn   int      `json:"expires_in"`
	User        UserInfo `json:"user"`
}

// LoginChallengeResponse is returned when a second factor (OTP) is required.
type LoginChallengeResponse struct {
	Status         string `json:"status"` // always "otp_required"
	ChallengeID    string `json:"challenge_id"`
	ExpiresIn      int    `json:"expires_in"`
	DeliveryMethod string `json:"delivery_method"`
	Destination    string `json:"destination"`
	Message        string `json:"message"`
}

// RegisterResponse is returned after a successful registration.
type RegisterResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	UserID  uint   `json:"user_id"`
	Email   string `json:"email"`
}

// EmailVerificationResponse is returned after verifying an email address.
type EmailVerificationResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ForgotPasswordResponse is returned from the forgot-password endpoint.
type ForgotPasswordResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ResetPasswordResponse is returned after a password reset.
type ResetPasswordResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
