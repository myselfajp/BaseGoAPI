package service

import (
	"fmt"
	"strings"

	"github.com/myselfajp/BaseGoAPI/internal/apperror"
	"github.com/myselfajp/BaseGoAPI/internal/core/pagination"
	"github.com/myselfajp/BaseGoAPI/internal/core/security"
	"github.com/myselfajp/BaseGoAPI/internal/core/validation"
	"github.com/myselfajp/BaseGoAPI/internal/dto"
	"github.com/myselfajp/BaseGoAPI/internal/model"
	"github.com/myselfajp/BaseGoAPI/internal/repository"
)

// validRoles is the set of roles the base template accepts. Extend it for your
// own application.
var validRoles = map[string]bool{
	model.RoleAdmin: true,
	model.RoleUser:  true,
}

func rolesList() string {
	return strings.Join([]string{model.RoleAdmin, model.RoleUser}, ", ")
}

// UserService holds the business logic for user management.
type UserService struct {
	userRepo *repository.UserRepository
}

// NewUserService builds a UserService.
func NewUserService(userRepo *repository.UserRepository) *UserService {
	return &UserService{userRepo: userRepo}
}

// CreateUser validates the input, hashes the password and persists a new user.
// New users are created unverified with 2FA disabled.
func (s *UserService) CreateUser(email, password, fullName, phoneNumber, role string, isActive bool) (*model.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if err := validation.ValidateEmail(email); err != nil {
		return nil, apperror.BadRequest(err.Error())
	}

	existing, err := s.userRepo.GetByEmail(email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, apperror.BadRequest(fmt.Sprintf("User with email %s already exists", email))
	}

	if err := validation.ValidatePassword(password); err != nil {
		return nil, apperror.BadRequest(err.Error())
	}
	if err := validation.ValidatePhoneNumber(phoneNumber); err != nil {
		return nil, apperror.BadRequest(err.Error())
	}

	fullName = validation.SanitizeString(fullName, 255)
	if fullName == "" {
		return nil, apperror.BadRequest("Full name is required")
	}

	if role == "" {
		role = model.RoleUser
	}
	if !validRoles[role] {
		return nil, apperror.BadRequest("Invalid role. Must be one of: " + rolesList())
	}

	hashed, err := security.HashPassword(password)
	if err != nil {
		return nil, apperror.Internal("Failed to hash password")
	}

	user := &model.User{
		Email:              email,
		PasswordHash:       hashed,
		FullName:           fullName,
		Role:               role,
		IsActive:           isActive,
		PhoneNumber:        phoneNumber,
		IsEmailVerified:    false,
		IsTwoFactorEnabled: false,
	}
	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}
	return user, nil
}

// ListUsers returns a paginated, filtered list of users.
func (s *UserService) ListUsers(f repository.UserFilters) (*dto.UserListResponse, error) {
	users, total, err := s.userRepo.ListWithFilters(f)
	if err != nil {
		return nil, err
	}

	items := make([]dto.UserListItem, 0, len(users))
	for _, u := range users {
		items = append(items, dto.UserListItem{
			ID:                 u.ID,
			FullName:           u.FullName,
			Email:              u.Email,
			PhoneNumber:        u.PhoneNumber,
			Role:               u.Role,
			IsActive:           u.IsActive,
			IsEmailVerified:    u.IsEmailVerified,
			IsTwoFactorEnabled: u.IsTwoFactorEnabled,
			CreatedAt:          u.CreatedAt,
			UpdatedAt:          u.UpdatedAt,
		})
	}

	return &dto.UserListResponse{
		Status: "success",
		Data: dto.UserListData{
			Users:      items,
			Total:      total,
			Page:       f.Page,
			Limit:      f.Limit,
			TotalPages: pagination.CalculateTotalPages(int(total), f.Limit),
		},
	}, nil
}

// GetUserByEmail fetches a user by email, returning (nil, nil) when not found.
func (s *UserService) GetUserByEmail(email string) (*model.User, error) {
	return s.userRepo.GetByEmail(strings.ToLower(strings.TrimSpace(email)))
}

// GetUser fetches a user by id, returning a 404 when it does not exist.
func (s *UserService) GetUser(userID uint) (*model.User, error) {
	user, err := s.userRepo.Get(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, apperror.NotFound(fmt.Sprintf("User with id %d not found", userID))
	}
	return user, nil
}

// UpdateUser applies the provided (optional) fields to a user. Changing the
// email address resets the verification flag; demoting the last admin or using
// an invalid role is rejected.
func (s *UserService) UpdateUser(userID uint, req dto.UserUpdateRequest) (*model.User, error) {
	user, err := s.GetUser(userID)
	if err != nil {
		return nil, err
	}

	changed := false

	if req.Email != nil {
		email := strings.ToLower(strings.TrimSpace(*req.Email))
		if email != user.Email {
			if err := validation.ValidateEmail(email); err != nil {
				return nil, apperror.BadRequest(err.Error())
			}
			existing, err := s.userRepo.GetByEmail(email)
			if err != nil {
				return nil, err
			}
			if existing != nil {
				return nil, apperror.BadRequest(fmt.Sprintf("User with email %s already exists", email))
			}
			user.Email = email
			user.IsEmailVerified = false // require re-verification
			changed = true
		}
	}

	if req.Password != nil {
		if err := validation.ValidatePassword(*req.Password); err != nil {
			return nil, apperror.BadRequest(err.Error())
		}
		hashed, err := security.HashPassword(*req.Password)
		if err != nil {
			return nil, apperror.Internal("Failed to hash password")
		}
		user.PasswordHash = hashed
		changed = true
	}

	if req.Role != nil {
		role := *req.Role
		if !validRoles[role] {
			return nil, apperror.BadRequest("Invalid role. Must be one of: " + rolesList())
		}
		// Protect the last remaining admin from being demoted.
		if user.Role == model.RoleAdmin && role != model.RoleAdmin {
			adminCount, err := s.userRepo.CountAdmins()
			if err != nil {
				return nil, err
			}
			if adminCount <= 1 {
				return nil, apperror.Forbidden(
					"Cannot change role: This is the last admin user. System must have at least one admin.",
				)
			}
		}
		user.Role = role
		changed = true
	}

	if req.FullName != nil {
		fullName := validation.SanitizeString(*req.FullName, 255)
		if fullName == "" {
			return nil, apperror.BadRequest("Full name cannot be empty")
		}
		user.FullName = fullName
		changed = true
	}

	if req.PhoneNumber != nil {
		if err := validation.ValidatePhoneNumber(*req.PhoneNumber); err != nil {
			return nil, apperror.BadRequest(err.Error())
		}
		user.PhoneNumber = *req.PhoneNumber
		changed = true
	}

	if req.IsActive != nil {
		user.IsActive = *req.IsActive
		changed = true
	}

	if req.IsTwoFactorEnabled != nil {
		user.IsTwoFactorEnabled = *req.IsTwoFactorEnabled
		changed = true
	}

	if !changed {
		return user, nil
	}

	if err := s.userRepo.Save(user); err != nil {
		return nil, err
	}
	return user, nil
}

// VerifyEmail marks a user's email address as verified.
func (s *UserService) VerifyEmail(userID uint) (*model.User, error) {
	user, err := s.GetUser(userID)
	if err != nil {
		return nil, err
	}
	user.IsEmailVerified = true
	if err := s.userRepo.Save(user); err != nil {
		return nil, err
	}
	return user, nil
}

// DeleteUser removes a user. The caller cannot delete their own account nor the
// last remaining admin.
func (s *UserService) DeleteUser(userID uint, currentUser *model.User) error {
	user, err := s.GetUser(userID)
	if err != nil {
		return err
	}

	if user.ID == currentUser.ID {
		return apperror.Forbidden("Cannot delete your own account")
	}

	if user.Role == model.RoleAdmin {
		adminCount, err := s.userRepo.CountAdmins()
		if err != nil {
			return err
		}
		if adminCount <= 1 {
			return apperror.Forbidden(
				"Cannot delete the last admin user. System must have at least one admin.",
			)
		}
	}

	return s.userRepo.Delete(user)
}
