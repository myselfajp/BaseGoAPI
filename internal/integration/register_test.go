package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/myselfajp/BaseGoAPI/internal/model"
)

// errDetail decodes the standard error envelope ({"detail": "..."}) and returns
// the message so failure cases can assert on it.
func errDetail(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Detail string `json:"detail"`
	}
	decodeBody(t, rec, &body)
	return body.Detail
}

// validRegisterBody returns a fresh, fully-valid registration payload. Callers
// mutate the returned map to craft individual (in)valid cases.
func validRegisterBody() map[string]any {
	return map[string]any{
		"email":        "fresh@test.local",
		"password":     "Password1",
		"full_name":    "Fresh User",
		"phone_number": "+1234567890",
	}
}

// TestRegisterSuccess covers the happy path: a 201 with the documented response
// envelope, exactly one verification email carrying a token, and a freshly
// persisted user that is a plain "user" and not yet email-verified.
func TestRegisterSuccess(t *testing.T) {
	app := newTestApp(t)

	rec := app.request(http.MethodPost, "/v1/auth/register", map[string]any{
		"email":        "bob@test.local",
		"password":     "Password1",
		"full_name":    "Bob Tester",
		"phone_number": "+1234567890",
	}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: got %d, want 201; body %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		UserID  uint   `json:"user_id"`
		Email   string `json:"email"`
	}
	decodeBody(t, rec, &resp)
	if resp.Status != "success" {
		t.Errorf("register status: got %q, want %q; body %s", resp.Status, "success", rec.Body.String())
	}
	if resp.UserID == 0 {
		t.Errorf("register user_id: got 0, want non-zero; body %s", rec.Body.String())
	}
	if resp.Email != "bob@test.local" {
		t.Errorf("register email: got %q, want %q; body %s", resp.Email, "bob@test.local", rec.Body.String())
	}

	// Exactly one verification email whose BodyText contains a token.
	if got := app.mail.count(); got != 1 {
		t.Fatalf("verification emails sent: got %d, want 1", got)
	}
	if tok := extractVerifyToken(t, app.mail.last().BodyText); tok == "" {
		t.Fatalf("verification email did not contain a token; body: %s", app.mail.last().BodyText)
	}

	// The persisted user is a plain, unverified "user".
	var u model.User
	if err := app.db.Where("email = ?", "bob@test.local").First(&u).Error; err != nil {
		t.Fatalf("failed to read registered user: %v", err)
	}
	if u.Role != model.RoleUser {
		t.Errorf("registered user role: got %q, want %q", u.Role, model.RoleUser)
	}
	if u.IsEmailVerified {
		t.Errorf("registered user is_email_verified: got true, want false")
	}
	if u.ID != resp.UserID {
		t.Errorf("registered user id: response %d != db %d", resp.UserID, u.ID)
	}
}

// TestRegisterValidationErrors exercises every rejection path for registration.
// All of them are expected to fail with a 400 and a non-empty detail message;
// the ones produced by the service layer are asserted verbatim.
func TestRegisterValidationErrors(t *testing.T) {
	app := newTestApp(t)

	// Pre-seed a user so the duplicate-email case has something to collide with.
	app.seedVerifiedUser("existing@test.local", "Password1", model.RoleUser)

	cases := []struct {
		name       string
		mutate     func(map[string]any)
		wantDetail string // "" => only assert the status code / that detail is present
	}{
		{
			name:       "duplicate email",
			mutate:     func(m map[string]any) { m["email"] = "existing@test.local" },
			wantDetail: "User with email existing@test.local already exists",
		},
		{
			// Rejected by the binding layer ("email" tag); message is gin-internal.
			name:   "invalid email format",
			mutate: func(m map[string]any) { m["email"] = "not-an-email" },
		},
		{
			name:       "weak password",
			mutate:     func(m map[string]any) { m["password"] = "short" },
			wantDetail: "Password must be at least 8 characters long",
		},
		{
			name:       "invalid phone number",
			mutate:     func(m map[string]any) { m["phone_number"] = "abc" },
			wantDetail: "Phone number must contain only digits, spaces, dashes, or start with +",
		},
		{
			// Rejected by the binding layer ("required" tag); message is gin-internal.
			name:   "missing full_name",
			mutate: func(m map[string]any) { delete(m, "full_name") },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := validRegisterBody()
			tc.mutate(body)

			rec := app.request(http.MethodPost, "/v1/auth/register", body, "")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%s: got %d, want 400; body %s", tc.name, rec.Code, rec.Body.String())
			}
			detail := errDetail(t, rec)
			if detail == "" {
				t.Fatalf("%s: expected a non-empty detail; body %s", tc.name, rec.Body.String())
			}
			if tc.wantDetail != "" && detail != tc.wantDetail {
				t.Errorf("%s: detail got %q, want %q", tc.name, detail, tc.wantDetail)
			}
		})
	}
}

// TestVerifyEmail covers the verify-email endpoint: a valid token flips the
// account to verified and is then single-use, and the various malformed / stale
// token inputs are all rejected with a 400.
func TestVerifyEmail(t *testing.T) {
	t.Run("valid token verifies and cannot be reused", func(t *testing.T) {
		app := newTestApp(t)

		rec := app.request(http.MethodPost, "/v1/auth/register", map[string]any{
			"email":        "carol@test.local",
			"password":     "Password1",
			"full_name":    "Carol",
			"phone_number": "+1234567890",
		}, "")
		if rec.Code != http.StatusCreated {
			t.Fatalf("register: got %d, want 201; body %s", rec.Code, rec.Body.String())
		}
		token := extractVerifyToken(t, app.mail.last().BodyText)

		rec = app.request(http.MethodPost, "/v1/auth/verify-email?token="+token, nil, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("verify-email: got %d, want 200; body %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}
		decodeBody(t, rec, &resp)
		if resp.Status != "success" {
			t.Errorf("verify-email status: got %q, want %q; body %s", resp.Status, "success", rec.Body.String())
		}

		// The account is now verified in the database.
		var u model.User
		if err := app.db.Where("email = ?", "carol@test.local").First(&u).Error; err != nil {
			t.Fatalf("failed to read verified user: %v", err)
		}
		if !u.IsEmailVerified {
			t.Errorf("user is_email_verified: got false, want true")
		}

		// Reusing the token fails: verification purges the token, so the second
		// lookup finds nothing and reports it as invalid.
		rec = app.request(http.MethodPost, "/v1/auth/verify-email?token="+token, nil, "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("verify-email reuse: got %d, want 400; body %s", rec.Code, rec.Body.String())
		}
		if got, want := errDetail(t, rec), "Invalid verification token."; got != want {
			t.Errorf("verify-email reuse detail: got %q, want %q", got, want)
		}
	})

	t.Run("unknown token", func(t *testing.T) {
		app := newTestApp(t)

		rec := app.request(http.MethodPost, "/v1/auth/verify-email?token=does-not-exist", nil, "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("verify-email unknown: got %d, want 400; body %s", rec.Code, rec.Body.String())
		}
		if got, want := errDetail(t, rec), "Invalid verification token."; got != want {
			t.Errorf("verify-email unknown detail: got %q, want %q", got, want)
		}
	})

	t.Run("missing token query param", func(t *testing.T) {
		app := newTestApp(t)

		rec := app.request(http.MethodPost, "/v1/auth/verify-email", nil, "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("verify-email missing token: got %d, want 400; body %s", rec.Code, rec.Body.String())
		}
		if got, want := errDetail(t, rec), "Email verification token is required"; got != want {
			t.Errorf("verify-email missing token detail: got %q, want %q", got, want)
		}
	})
}

// TestVerifyEmailExpiredToken inserts an already-expired verification token
// directly and confirms verify-email rejects it with a 400 without verifying the
// account.
func TestVerifyEmailExpiredToken(t *testing.T) {
	app := newTestApp(t)

	u := app.seedUser("stale@test.local", "Password1", model.RoleUser, false, false, true)

	// Insert a token whose ExpiresAt is in the past. Verification tokens are
	// stored in plaintext (GetByToken matches the raw value), so no hashing.
	expired := &model.EmailVerificationToken{
		UserID:    u.ID,
		Token:     "expired-token-value",
		ExpiresAt: time.Now().UTC().Add(-1 * time.Hour),
	}
	if err := app.db.Create(expired).Error; err != nil {
		t.Fatalf("failed to insert expired verification token: %v", err)
	}

	rec := app.request(http.MethodPost, "/v1/auth/verify-email?token=expired-token-value", nil, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("verify-email expired: got %d, want 400; body %s", rec.Code, rec.Body.String())
	}
	if got, want := errDetail(t, rec), "Verification token has expired. Please request a new verification email."; got != want {
		t.Errorf("verify-email expired detail: got %q, want %q", got, want)
	}

	// The account must remain unverified.
	var got model.User
	if err := app.db.First(&got, u.ID).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if got.IsEmailVerified {
		t.Errorf("expired token: user is_email_verified got true, want false")
	}
}
