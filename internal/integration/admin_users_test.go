package integration

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/myselfajp/BaseGoAPI/internal/model"
)

// userListResponse mirrors dto.UserListResponse for decoding.
type userListResponse struct {
	Status string `json:"status"`
	Data   struct {
		Users []struct {
			ID       uint   `json:"id"`
			Email    string `json:"email"`
			FullName string `json:"full_name"`
			Role     string `json:"role"`
			IsActive bool   `json:"is_active"`
		} `json:"users"`
		Total      int64 `json:"total"`
		Page       int   `json:"page"`
		Limit      int   `json:"limit"`
		TotalPages int   `json:"total_pages"`
	} `json:"data"`
}

// userDetailResponse mirrors dto.UserDetailResponse for decoding.
type userDetailResponse struct {
	Status          string `json:"status"`
	ID              uint   `json:"id"`
	Email           string `json:"email"`
	FullName        string `json:"full_name"`
	Role            string `json:"role"`
	IsActive        bool   `json:"is_active"`
	IsEmailVerified bool   `json:"is_email_verified"`
}

func TestAdminListUsersPaginationAndFilters(t *testing.T) {
	app := newTestApp(t)
	_, adminTok := app.seedAdminWithToken("admin@test.local", "AdminPass1")

	// Fixtures: u1..u3 are regular users; u2 is inactive.
	app.seedVerifiedUser("u1@test.local", "Password1", model.RoleUser)
	app.seedUser("u2@test.local", "Password1", model.RoleUser, true, false, false)
	u3 := app.seedVerifiedUser("u3@test.local", "Password1", model.RoleUser)

	// Give u3 a distinctive name for the search test.
	if err := app.db.Model(&model.User{}).Where("id = ?", u3.ID).
		Update("full_name", "Zebra Quirk").Error; err != nil {
		t.Fatalf("failed to rename u3: %v", err)
	}

	// Pagination: 4 users total, limit 2 -> 2 pages.
	rec := app.request(http.MethodGet, "/v1/admin/users?page=1&limit=2", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: got %d body %s", rec.Code, rec.Body.String())
	}
	var list userListResponse
	decodeBody(t, rec, &list)
	if list.Data.Total != 4 || list.Data.TotalPages != 2 || len(list.Data.Users) != 2 {
		t.Fatalf("pagination: total=%d pages=%d len=%d, want 4/2/2 (body %s)",
			list.Data.Total, list.Data.TotalPages, len(list.Data.Users), rec.Body.String())
	}

	// Search by full name (case-insensitive).
	rec = app.request(http.MethodGet, "/v1/admin/users?search=zebra", nil, adminTok)
	decodeBody(t, rec, &list)
	if list.Data.Total != 1 || list.Data.Users[0].Email != "u3@test.local" {
		t.Fatalf("search=zebra: got %s", rec.Body.String())
	}

	// Search by email fragment.
	rec = app.request(http.MethodGet, "/v1/admin/users?search=u1@", nil, adminTok)
	decodeBody(t, rec, &list)
	if list.Data.Total != 1 || list.Data.Users[0].Email != "u1@test.local" {
		t.Fatalf("search=u1@: got %s", rec.Body.String())
	}

	// Filter by role.
	rec = app.request(http.MethodGet, "/v1/admin/users?role=admin", nil, adminTok)
	decodeBody(t, rec, &list)
	if list.Data.Total != 1 || list.Data.Users[0].Email != "admin@test.local" {
		t.Fatalf("role=admin: got %s", rec.Body.String())
	}

	// Filter by active status.
	rec = app.request(http.MethodGet, "/v1/admin/users?is_active=false", nil, adminTok)
	decodeBody(t, rec, &list)
	if list.Data.Total != 1 || list.Data.Users[0].Email != "u2@test.local" {
		t.Fatalf("is_active=false: got %s", rec.Body.String())
	}

	// Sorting: role=user + email desc is deterministic regardless of collation.
	rec = app.request(http.MethodGet,
		"/v1/admin/users?role=user&sort_by=email&sort_order=desc", nil, adminTok)
	decodeBody(t, rec, &list)
	if len(list.Data.Users) != 3 || list.Data.Users[0].Email != "u3@test.local" ||
		list.Data.Users[2].Email != "u1@test.local" {
		t.Fatalf("sort email desc: got %s", rec.Body.String())
	}
}

func TestAdminCreateUser(t *testing.T) {
	app := newTestApp(t)
	_, adminTok := app.seedAdminWithToken("admin@test.local", "AdminPass1")

	// Role omitted -> defaults to "user"; new users start unverified.
	rec := app.request(http.MethodPost, "/v1/admin/users", map[string]any{
		"email":        "new@test.local",
		"password":     "Password1",
		"full_name":    "New User",
		"phone_number": "+1234567890",
	}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: got %d body %s", rec.Code, rec.Body.String())
	}
	var created userDetailResponse
	decodeBody(t, rec, &created)
	if created.Role != model.RoleUser || created.IsEmailVerified || !created.IsActive {
		t.Fatalf("create defaults: %+v", created)
	}

	// Regression: creating an inactive user must actually persist is_active =
	// false (a gorm `default:true` tag used to drop the false from the INSERT).
	rec = app.request(http.MethodPost, "/v1/admin/users", map[string]any{
		"email":        "inactive@test.local",
		"password":     "Password1",
		"full_name":    "Inactive User",
		"phone_number": "+1234567890",
		"is_active":    false,
	}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create inactive: got %d body %s", rec.Code, rec.Body.String())
	}
	decodeBody(t, rec, &created)
	rec = app.request(http.MethodGet, fmt.Sprintf("/v1/admin/users/%d", created.ID), nil, adminTok)
	var fetched userDetailResponse
	decodeBody(t, rec, &fetched)
	if fetched.IsActive {
		t.Fatalf("inactive user was stored as active: %s", rec.Body.String())
	}

	// Duplicate email is rejected.
	rec = app.request(http.MethodPost, "/v1/admin/users", map[string]any{
		"email":        "new@test.local",
		"password":     "Password1",
		"full_name":    "Dup User",
		"phone_number": "+1234567890",
	}, adminTok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("duplicate email: expected 400, got %d body %s", rec.Code, rec.Body.String())
	}

	// Roles from the old template ("editor") no longer exist.
	rec = app.request(http.MethodPost, "/v1/admin/users", map[string]any{
		"email":        "editor@test.local",
		"password":     "Password1",
		"full_name":    "Old Role",
		"phone_number": "+1234567890",
		"role":         "editor",
	}, adminTok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid role: expected 400, got %d body %s", rec.Code, rec.Body.String())
	}

	// Weak password is rejected.
	rec = app.request(http.MethodPost, "/v1/admin/users", map[string]any{
		"email":        "weak@test.local",
		"password":     "weak",
		"full_name":    "Weak Pass",
		"phone_number": "+1234567890",
	}, adminTok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("weak password: expected 400, got %d body %s", rec.Code, rec.Body.String())
	}
}

func TestAdminGetUser(t *testing.T) {
	app := newTestApp(t)
	_, adminTok := app.seedAdminWithToken("admin@test.local", "AdminPass1")
	u := app.seedVerifiedUser("target@test.local", "Password1", model.RoleUser)

	rec := app.request(http.MethodGet, fmt.Sprintf("/v1/admin/users/%d", u.ID), nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: got %d body %s", rec.Code, rec.Body.String())
	}
	var got userDetailResponse
	decodeBody(t, rec, &got)
	if got.Email != "target@test.local" {
		t.Fatalf("get: unexpected user %+v", got)
	}

	rec = app.request(http.MethodGet, "/v1/admin/users/999999", nil, adminTok)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get missing: expected 404, got %d body %s", rec.Code, rec.Body.String())
	}

	rec = app.request(http.MethodGet, "/v1/admin/users/not-a-number", nil, adminTok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("get bad id: expected 400, got %d body %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUpdateUser(t *testing.T) {
	app := newTestApp(t)
	admin, adminTok := app.seedAdminWithToken("admin@test.local", "AdminPass1")
	u := app.seedVerifiedUser("victim@test.local", "Password1", model.RoleUser)

	// Changing the email resets the verification flag.
	rec := app.request(http.MethodPut, fmt.Sprintf("/v1/admin/users/%d", u.ID),
		map[string]any{"email": "renamed@test.local"}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("update email: got %d body %s", rec.Code, rec.Body.String())
	}
	var updated userDetailResponse
	decodeBody(t, rec, &updated)
	if updated.Email != "renamed@test.local" || updated.IsEmailVerified {
		t.Fatalf("update email: verification flag should reset, got %+v", updated)
	}

	// Weak password is rejected.
	rec = app.request(http.MethodPut, fmt.Sprintf("/v1/admin/users/%d", u.ID),
		map[string]any{"password": "weak"}, adminTok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("weak password: expected 400, got %d body %s", rec.Code, rec.Body.String())
	}

	// Invalid role is rejected.
	rec = app.request(http.MethodPut, fmt.Sprintf("/v1/admin/users/%d", u.ID),
		map[string]any{"role": "boss"}, adminTok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid role: expected 400, got %d body %s", rec.Code, rec.Body.String())
	}

	// Demoting the only admin is blocked.
	rec = app.request(http.MethodPut, fmt.Sprintf("/v1/admin/users/%d", admin.ID),
		map[string]any{"role": model.RoleUser}, adminTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("demote last admin: expected 403, got %d body %s", rec.Code, rec.Body.String())
	}

	// With a second admin present, demotion succeeds.
	admin2 := app.seedVerifiedUser("admin2@test.local", "AdminPass1", model.RoleAdmin)
	rec = app.request(http.MethodPut, fmt.Sprintf("/v1/admin/users/%d", admin2.ID),
		map[string]any{"role": model.RoleUser}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("demote second admin: got %d body %s", rec.Code, rec.Body.String())
	}
	decodeBody(t, rec, &updated)
	if updated.Role != model.RoleUser {
		t.Fatalf("demote second admin: role = %q, want %q", updated.Role, model.RoleUser)
	}
}

func TestAdminDeleteUser(t *testing.T) {
	app := newTestApp(t)
	admin, adminTok := app.seedAdminWithToken("admin@test.local", "AdminPass1")
	u := app.seedVerifiedUser("doomed@test.local", "Password1", model.RoleUser)
	doomedTok := app.tokenFor("doomed@test.local")

	// Self-deletion is blocked.
	rec := app.request(http.MethodDelete, fmt.Sprintf("/v1/admin/users/%d", admin.ID), nil, adminTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("self delete: expected 403, got %d body %s", rec.Code, rec.Body.String())
	}

	// Deleting a missing user is a 404.
	rec = app.request(http.MethodDelete, "/v1/admin/users/999999", nil, adminTok)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete missing: expected 404, got %d body %s", rec.Code, rec.Body.String())
	}

	// Normal deletion succeeds and the user is gone.
	rec = app.request(http.MethodDelete, fmt.Sprintf("/v1/admin/users/%d", u.ID), nil, adminTok)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d body %s", rec.Code, rec.Body.String())
	}
	rec = app.request(http.MethodGet, fmt.Sprintf("/v1/admin/users/%d", u.ID), nil, adminTok)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete: expected 404, got %d body %s", rec.Code, rec.Body.String())
	}

	// A still-valid JWT for the deleted user no longer authenticates.
	rec = app.request(http.MethodGet, "/v1/admin/users", nil, doomedTok)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("deleted user token: expected 401, got %d body %s", rec.Code, rec.Body.String())
	}
}
