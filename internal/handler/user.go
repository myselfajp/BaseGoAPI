package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/myselfajp/BaseGoAPI/internal/dto"
	"github.com/myselfajp/BaseGoAPI/internal/middleware"
	"github.com/myselfajp/BaseGoAPI/internal/model"
	"github.com/myselfajp/BaseGoAPI/internal/repository"
	"github.com/myselfajp/BaseGoAPI/internal/service"
)

// UserHandler exposes the admin user-management endpoints.
type UserHandler struct {
	userService *service.UserService
}

// NewUserHandler builds a UserHandler.
func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// ListUsers handles GET /v1/admin/users.
func (h *UserHandler) ListUsers(c *gin.Context) {
	page := parseIntDefault(c.Query("page"), 1)
	if page < 1 {
		page = 1
	}
	limit := parseIntDefault(c.Query("limit"), 20)
	if limit < 1 || limit > 100 {
		limit = 20
	}

	filters := repository.UserFilters{
		Page:      page,
		Limit:     limit,
		Search:    c.Query("search"),
		Role:      c.Query("role"),
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}
	if raw := c.Query("is_active"); raw != "" {
		if b, err := strconv.ParseBool(raw); err == nil {
			filters.IsActive = &b
		}
	}

	resp, err := h.userService.ListUsers(filters)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetUser handles GET /v1/admin/users/:id.
func (h *UserHandler) GetUser(c *gin.Context) {
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	user, err := h.userService.GetUser(id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toUserDetail(user))
}

// CreateUser handles POST /v1/admin/users.
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req dto.UserCreateRequest
	if !bindJSON(c, &req) {
		return
	}

	role := req.Role
	if role == "" {
		role = model.RoleUser
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	user, err := h.userService.CreateUser(req.Email, req.Password, req.FullName, req.PhoneNumber, role, isActive)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toUserDetail(user))
}

// UpdateUser handles PUT /v1/admin/users/:id.
func (h *UserHandler) UpdateUser(c *gin.Context) {
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	var req dto.UserUpdateRequest
	if !bindJSON(c, &req) {
		return
	}

	user, err := h.userService.UpdateUser(id, req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toUserDetail(user))
}

// DeleteUser handles DELETE /v1/admin/users/:id.
func (h *UserHandler) DeleteUser(c *gin.Context) {
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	currentUser := middleware.CurrentUser(c)
	if currentUser == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Could not validate credentials"})
		return
	}

	if err := h.userService.DeleteUser(id, currentUser); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// --- helpers ---

func toUserDetail(u *model.User) dto.UserDetailResponse {
	return dto.UserDetailResponse{
		Status:             "success",
		ID:                 u.ID,
		Email:              u.Email,
		FullName:           u.FullName,
		PhoneNumber:        u.PhoneNumber,
		Role:               u.Role,
		IsActive:           u.IsActive,
		IsEmailVerified:    u.IsEmailVerified,
		IsTwoFactorEnabled: u.IsTwoFactorEnabled,
		CreatedAt:          u.CreatedAt,
		UpdatedAt:          u.UpdatedAt,
	}
}

func parseIntDefault(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func parseUserID(c *gin.Context) (uint, bool) {
	raw := c.Param("id")
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid user id"})
		return 0, false
	}
	return uint(v), true
}
