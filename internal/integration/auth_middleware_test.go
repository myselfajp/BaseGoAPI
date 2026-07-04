package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/myselfajp/BaseGoAPI/internal/core/jwtutil"
	"github.com/myselfajp/BaseGoAPI/internal/dto"
	"github.com/myselfajp/BaseGoAPI/internal/model"
)

// errorResponse mirrors the {"detail": "..."} shape emitted by the auth/admin
// guard middleware (see internal/middleware/auth.go: abort).
type errorResponse struct {
	Detail string `json:"detail"`
}

// TestAuthMiddlewareRejectsUnauthorized exercises every 401 path of the Auth
// middleware guarding GET /v1/admin/users: a missing or malformed Authorization
// header, a syntactically invalid token, a token signed with the wrong secret,
// and a validly-signed token whose subject no longer exists in the database.
//
// The exact detail strings come from internal/middleware/auth.go:
//   - extractBearerToken failure -> "Authorization header missing or invalid"
//   - jwtutil.ParseSubject failure -> "Could not validate credentials"
//   - user not found in DB       -> "User not found"
func TestAuthMiddlewareRejectsUnauthorized(t *testing.T) {
	app := newTestApp(t)

	// A validly-signed token whose subject was never seeded: the signature is
	// valid (minted with app.cfg) but userRepo.GetByEmail returns (nil, nil),
	// so the middleware reports "User not found".
	ghostToken := app.tokenFor("ghost@test.local")

	// A token signed with a different secret than the server uses. The signature
	// check in jwtutil.ParseSubject fails -> "Could not validate credentials".
	wrongCfg := baseConfig()
	wrongCfg.JWTSecretKey = "a-completely-different-secret-key"
	wrongSecretToken, err := jwtutil.CreateAccessToken(wrongCfg, "ghost@test.local", 0)
	if err != nil {
		t.Fatalf("failed to mint wrong-secret token: %v", err)
	}

	tests := []struct {
		name       string
		setHeader  bool
		header     string // full Authorization header value when setHeader is true
		wantDetail string
	}{
		{
			name:       "no authorization header",
			setHeader:  false,
			wantDetail: "Authorization header missing or invalid",
		},
		{
			name:       "missing bearer scheme",
			setHeader:  true,
			header:     ghostToken, // raw token, no "Bearer " prefix
			wantDetail: "Authorization header missing or invalid",
		},
		{
			name:       "wrong scheme",
			setHeader:  true,
			header:     "Basic " + ghostToken,
			wantDetail: "Authorization header missing or invalid",
		},
		{
			name:       "empty bearer token",
			setHeader:  true,
			header:     "Bearer ",
			wantDetail: "Authorization header missing or invalid",
		},
		{
			name:       "garbage token",
			setHeader:  true,
			header:     "Bearer not-a-real-jwt",
			wantDetail: "Could not validate credentials",
		},
		{
			name:       "wrong secret",
			setHeader:  true,
			header:     "Bearer " + wrongSecretToken,
			wantDetail: "Could not validate credentials",
		},
		{
			name:       "valid token for deleted user",
			setHeader:  true,
			header:     "Bearer " + ghostToken,
			wantDetail: "User not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/admin/users", nil)
			if tc.setHeader {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			app.router.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("%s: got status %d, want %d; body=%s",
					tc.name, rec.Code, http.StatusUnauthorized, rec.Body.String())
			}

			var resp errorResponse
			decodeBody(t, rec, &resp)
			if resp.Detail != tc.wantDetail {
				t.Errorf("%s: got detail %q, want %q; body=%s",
					tc.name, resp.Detail, tc.wantDetail, rec.Body.String())
			}
		})
	}
}

// TestAdminGuardEnforcesRole verifies the AdminOnly middleware that runs after
// Auth on GET /v1/admin/users: an authenticated non-admin is rejected with 403
// and the exact "Access denied" detail, while an authenticated admin reaches
// the handler and receives the 200 user-list payload.
func TestAdminGuardEnforcesRole(t *testing.T) {
	app := newTestApp(t)

	_, adminToken := app.seedAdminWithToken("admin@test.local", "Password1")
	app.seedVerifiedUser("user@test.local", "Password1", model.RoleUser)
	userToken := app.tokenFor("user@test.local")

	t.Run("non-admin is forbidden", func(t *testing.T) {
		rec := app.request(http.MethodGet, "/v1/admin/users", nil, userToken)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("non-admin: got status %d, want %d; body=%s",
				rec.Code, http.StatusForbidden, rec.Body.String())
		}

		var resp errorResponse
		decodeBody(t, rec, &resp)
		const want = "Access denied. Admin privileges required"
		if resp.Detail != want {
			t.Errorf("non-admin: got detail %q, want %q; body=%s",
				resp.Detail, want, rec.Body.String())
		}
	})

	t.Run("admin is allowed", func(t *testing.T) {
		rec := app.request(http.MethodGet, "/v1/admin/users", nil, adminToken)
		if rec.Code != http.StatusOK {
			t.Fatalf("admin: got status %d, want %d; body=%s",
				rec.Code, http.StatusOK, rec.Body.String())
		}

		var resp dto.UserListResponse
		decodeBody(t, rec, &resp)
		if resp.Status != "success" {
			t.Errorf("admin: got status field %q, want %q; body=%s",
				resp.Status, "success", rec.Body.String())
		}
		// Both seeded users (admin + user) must show up in the listing.
		if resp.Data.Total < 2 {
			t.Errorf("admin: got total %d, want >= 2; body=%s",
				resp.Data.Total, rec.Body.String())
		}
		if len(resp.Data.Users) < 2 {
			t.Errorf("admin: got %d users in page, want >= 2; body=%s",
				len(resp.Data.Users), rec.Body.String())
		}
	})
}
