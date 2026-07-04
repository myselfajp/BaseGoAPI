package dto

import "time"

// --- Requests ---

// UserCreateRequest is the body of POST /v1/admin/users. Role and IsActive are
// optional; when omitted they default to "user" and true respectively.
type UserCreateRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required"`
	FullName    string `json:"full_name" binding:"required"`
	PhoneNumber string `json:"phone_number" binding:"required"`
	Role        string `json:"role"`
	IsActive    *bool  `json:"is_active"`
}

// UserUpdateRequest is the body of PUT /v1/admin/users/:id. Every field is
// optional; only the provided fields are updated.
type UserUpdateRequest struct {
	Email              *string `json:"email"`
	Password           *string `json:"password"`
	FullName           *string `json:"full_name"`
	PhoneNumber        *string `json:"phone_number"`
	Role               *string `json:"role"`
	IsActive           *bool   `json:"is_active"`
	IsTwoFactorEnabled *bool   `json:"is_two_factor_enabled"`
}

// --- Responses ---

// UserListItem is one row of the admin user list.
type UserListItem struct {
	ID                 uint      `json:"id"`
	FullName           string    `json:"full_name"`
	Email              string    `json:"email"`
	PhoneNumber        string    `json:"phone_number"`
	Role               string    `json:"role"`
	IsActive           bool      `json:"is_active"`
	IsEmailVerified    bool      `json:"is_email_verified"`
	IsTwoFactorEnabled bool      `json:"is_two_factor_enabled"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// UserListData is the paginated payload of the user list response.
type UserListData struct {
	Users      []UserListItem `json:"users"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	Limit      int            `json:"limit"`
	TotalPages int            `json:"total_pages"`
}

// UserListResponse wraps the user list data.
type UserListResponse struct {
	Status string       `json:"status"`
	Data   UserListData `json:"data"`
}

// UserDetailResponse is returned by GET/POST/PUT on a single user.
type UserDetailResponse struct {
	Status             string    `json:"status"`
	ID                 uint      `json:"id"`
	Email              string    `json:"email"`
	FullName           string    `json:"full_name"`
	PhoneNumber        string    `json:"phone_number"`
	Role               string    `json:"role"`
	IsActive           bool      `json:"is_active"`
	IsEmailVerified    bool      `json:"is_email_verified"`
	IsTwoFactorEnabled bool      `json:"is_two_factor_enabled"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
