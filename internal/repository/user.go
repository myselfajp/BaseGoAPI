package repository

import (
	"errors"

	"gorm.io/gorm"

	"github.com/myselfajp/BaseGoAPI/internal/core/pagination"
	"github.com/myselfajp/BaseGoAPI/internal/model"
)

// UserFilters bundles the optional filtering, sorting and pagination options
// for listing users.
type UserFilters struct {
	Page      int
	Limit     int
	Search    string
	Role      string
	IsActive  *bool
	SortBy    string
	SortOrder string
}

// UserRepository is the data-access layer for users.
type UserRepository struct {
	*BaseRepository[model.User]
}

// NewUserRepository builds a UserRepository.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{BaseRepository: NewBaseRepository[model.User](db, "User")}
}

// GetByEmail returns the user with the given email, or (nil, nil) when none.
func (r *UserRepository) GetByEmail(email string) (*model.User, error) {
	var user model.User
	err := r.DB().Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// CountAdmins returns the number of users with the admin role.
func (r *UserRepository) CountAdmins() (int64, error) {
	var count int64
	err := r.DB().Model(&model.User{}).Where("role = ?", model.RoleAdmin).Count(&count).Error
	return count, err
}

// allowedSortFields guards the ORDER BY clause against injection.
var allowedSortFields = map[string]bool{
	"id":         true,
	"email":      true,
	"full_name":  true,
	"role":       true,
	"created_at": true,
	"updated_at": true,
}

// ListWithFilters returns a filtered, sorted page of users and the total count
// matching the filters (before pagination).
func (r *UserRepository) ListWithFilters(f UserFilters) ([]model.User, int64, error) {
	sortBy := f.SortBy
	if !allowedSortFields[sortBy] {
		sortBy = "created_at"
	}
	direction := "DESC"
	if f.SortOrder == "asc" {
		direction = "ASC"
	}

	query := r.DB().Model(&model.User{})

	if f.Search != "" {
		term := "%" + f.Search + "%"
		query = query.Where("email ILIKE ? OR full_name ILIKE ?", term, term)
	}
	if f.Role != "" {
		query = query.Where("role = ?", f.Role)
	}
	if f.IsActive != nil {
		query = query.Where("is_active = ?", *f.IsActive)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []model.User
	offset := pagination.CalculateOffset(f.Page, f.Limit)
	err := query.
		Order(sortBy + " " + direction).
		Limit(f.Limit).
		Offset(offset).
		Find(&users).Error
	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}
