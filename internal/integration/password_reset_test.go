package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/myselfajp/BaseGoAPI/internal/model"
)

// Exact, source-of-truth response strings (see internal/handler/auth.go and
// internal/service/password_reset.go). Keeping them as constants makes the
// enumeration-prevention assertion (identical message for existing vs unknown
// accounts) impossible to get wrong.
const (
	forgotPasswordMessage = "If an account with that email exists, a password reset link has been sent."
	resetSuccessMessage   = "Password has been reset successfully. You can now log in with your new password."

	// After a successful reset the token row is purged, and unknown tokens are
	// never stored, so both collapse onto the same generic message.
	invalidResetTokenMessage = "Invalid reset token."
	expiredResetTokenMessage = "Reset token has expired. Please request a new password reset."
)

// messageResponse matches the {status, message} envelope used by the
// forgot/reset endpoints on success.
type messageResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// detailResponse matches the {detail} envelope produced by respondError and the
// inline validation errors in the auth handler.
type detailResponse struct {
	Detail string `json:"detail"`
}

// TestForgotPassword covers the forgot-password endpoint, including the
// enumeration-prevention guarantee: an existing and a non-existent address must
// yield byte-for-byte the same 200 response, but only the existing account may
// trigger an email.
func TestForgotPassword(t *testing.T) {
	t.Run("existing user receives exactly one reset email", func(t *testing.T) {
		app := newTestApp(t)
		app.seedVerifiedUser("reset-me@test.local", "Password1", model.RoleUser)

		rec := app.request(http.MethodPost, "/v1/auth/forgot-password", map[string]any{
			"email": "reset-me@test.local",
		}, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("forgot-password: got status %d body %s, want 200", rec.Code, rec.Body.String())
		}

		var resp messageResponse
		decodeBody(t, rec, &resp)
		if resp.Message != forgotPasswordMessage {
			t.Errorf("forgot-password message: got %q, want %q", resp.Message, forgotPasswordMessage)
		}

		if got := app.mail.count(); got != 1 {
			t.Fatalf("emails sent: got %d, want 1 (body %s)", got, rec.Body.String())
		}
		mail := app.mail.last()
		if mail.Recipient != "reset-me@test.local" {
			t.Errorf("reset email recipient: got %q, want %q", mail.Recipient, "reset-me@test.local")
		}
		// extractResetToken fails the test if the body carries no reset token.
		if token := extractResetToken(t, mail.BodyText); token == "" {
			t.Errorf("reset email body carried an empty token: %s", mail.BodyText)
		}
	})

	t.Run("nonexistent email is indistinguishable and sends nothing", func(t *testing.T) {
		app := newTestApp(t)

		rec := app.request(http.MethodPost, "/v1/auth/forgot-password", map[string]any{
			"email": "nobody@test.local",
		}, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("forgot-password (unknown): got status %d body %s, want 200", rec.Code, rec.Body.String())
		}

		var resp messageResponse
		decodeBody(t, rec, &resp)
		if resp.Message != forgotPasswordMessage {
			t.Errorf("forgot-password (unknown) message: got %q, want %q (must match the existing-user message)",
				resp.Message, forgotPasswordMessage)
		}

		if got := app.mail.count(); got != 0 {
			t.Fatalf("emails sent for unknown account: got %d, want 0 (enumeration leak)", got)
		}
	})
}

// TestResetPasswordSuccessThenLogin walks the full happy path: request a token,
// consume it with a strong new password, then prove the credential swap took
// effect (new password logs in, old password is rejected).
func TestResetPasswordSuccessThenLogin(t *testing.T) {
	app := newTestApp(t)

	const (
		email       = "swap@test.local"
		oldPassword = "OldPassword1"
		newPassword = "BrandNewPass2"
	)
	app.seedVerifiedUser(email, oldPassword, model.RoleUser)

	// Request the reset token via the real endpoint and pull it from the email.
	rec := app.request(http.MethodPost, "/v1/auth/forgot-password", map[string]any{
		"email": email,
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("forgot-password: got status %d body %s, want 200", rec.Code, rec.Body.String())
	}
	token := extractResetToken(t, app.mail.last().BodyText)

	// Consume the token with a strong password.
	rec = app.request(http.MethodPost, "/v1/auth/reset-password", map[string]any{
		"token":        token,
		"new_password": newPassword,
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("reset-password: got status %d body %s, want 200", rec.Code, rec.Body.String())
	}
	var resp messageResponse
	decodeBody(t, rec, &resp)
	if resp.Message != resetSuccessMessage {
		t.Errorf("reset-password message: got %q, want %q", resp.Message, resetSuccessMessage)
	}

	// The NEW password now authenticates.
	rec = app.request(http.MethodPost, "/v1/auth/login", map[string]any{
		"email": email, "password": newPassword,
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login with new password: got status %d body %s, want 200", rec.Code, rec.Body.String())
	}

	// The OLD password no longer works (401 Incorrect credentials for a
	// non-admin's first wrong attempt).
	rec = app.request(http.MethodPost, "/v1/auth/login", map[string]any{
		"email": email, "password": oldPassword,
	}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("login with old password: got status %d body %s, want 401", rec.Code, rec.Body.String())
	}
}

// TestResetPasswordRejections exhaustively covers the 400 paths. Each case wires
// a fresh app (newTestApp truncates the DB) and returns the token + new password
// to submit, plus the exact detail message the endpoint must return.
func TestResetPasswordRejections(t *testing.T) {
	const strongPassword = "StrongPass9"

	tests := []struct {
		name       string
		setup      func(t *testing.T, app *testApp) (token, newPassword string)
		wantDetail string
	}{
		{
			name: "unknown token",
			setup: func(t *testing.T, app *testApp) (string, string) {
				// No token was ever issued for anyone.
				return "this-token-does-not-exist", strongPassword
			},
			wantDetail: invalidResetTokenMessage,
		},
		{
			name: "reused token after a successful reset",
			setup: func(t *testing.T, app *testApp) (string, string) {
				app.seedVerifiedUser("reuse@test.local", "Password1", model.RoleUser)
				rec := app.request(http.MethodPost, "/v1/auth/forgot-password", map[string]any{
					"email": "reuse@test.local",
				}, "")
				if rec.Code != http.StatusOK {
					t.Fatalf("setup forgot-password: got %d body %s", rec.Code, rec.Body.String())
				}
				token := extractResetToken(t, app.mail.last().BodyText)

				// First reset succeeds and consumes/purges the token.
				rec = app.request(http.MethodPost, "/v1/auth/reset-password", map[string]any{
					"token":        token,
					"new_password": strongPassword,
				}, "")
				if rec.Code != http.StatusOK {
					t.Fatalf("setup first reset: got %d body %s, want 200", rec.Code, rec.Body.String())
				}
				// Returning the same token drives the second (reuse) attempt.
				return token, "AnotherStrong3"
			},
			wantDetail: invalidResetTokenMessage,
		},
		{
			name: "expired token",
			setup: func(t *testing.T, app *testApp) (string, string) {
				u := app.seedVerifiedUser("expired@test.local", "Password1", model.RoleUser)
				// Insert a reset token row directly with an already-past expiry.
				// Reset tokens are stored in plaintext (matched via token = ?),
				// so no hashing is needed here.
				expired := &model.PasswordResetToken{
					UserID:    u.ID,
					Token:     "expired-token-value",
					ExpiresAt: time.Now().UTC().Add(-1 * time.Hour),
				}
				if err := app.db.Create(expired).Error; err != nil {
					t.Fatalf("setup expired token insert: %v", err)
				}
				return expired.Token, strongPassword
			},
			wantDetail: expiredResetTokenMessage,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestApp(t)
			token, newPassword := tc.setup(t, app)

			rec := app.request(http.MethodPost, "/v1/auth/reset-password", map[string]any{
				"token":        token,
				"new_password": newPassword,
			}, "")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("reset-password: got status %d body %s, want 400", rec.Code, rec.Body.String())
			}
			var resp detailResponse
			decodeBody(t, rec, &resp)
			if resp.Detail != tc.wantDetail {
				t.Errorf("reset-password detail: got %q, want %q", resp.Detail, tc.wantDetail)
			}
		})
	}
}

// TestResetPasswordWeakPasswordRejectedBeforeToken proves the password strength
// policy is enforced before the reset token is looked up or consumed:
//   - each weak password yields a 400 whose detail is the validation message
//     (never "Invalid reset token."), even though a bogus token is supplied;
//   - a real, still-valid token survives a rejected weak attempt and can then be
//     consumed with a strong password.
func TestResetPasswordWeakPasswordRejectedBeforeToken(t *testing.T) {
	weakCases := []struct {
		name       string
		password   string
		wantDetail string
	}{
		{"too short", "Ab1", "Password must be at least 8 characters long"},
		{"no uppercase", "password1", "Password must contain at least one uppercase letter"},
		{"no lowercase", "PASSWORD1", "Password must contain at least one lowercase letter"},
		{"no digit", "PasswordOnly", "Password must contain at least one digit"},
	}

	for _, tc := range weakCases {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestApp(t)

			// A deliberately bogus token: if the token were checked first this
			// would surface "Invalid reset token." instead of the password
			// validation message, so asserting the validation message proves
			// ordering.
			rec := app.request(http.MethodPost, "/v1/auth/reset-password", map[string]any{
				"token":        "irrelevant-token",
				"new_password": tc.password,
			}, "")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("reset-password (weak): got status %d body %s, want 400", rec.Code, rec.Body.String())
			}
			var resp detailResponse
			decodeBody(t, rec, &resp)
			if resp.Detail != tc.wantDetail {
				t.Errorf("reset-password (weak) detail: got %q, want %q", resp.Detail, tc.wantDetail)
			}
		})
	}

	t.Run("valid token is not consumed by a rejected weak attempt", func(t *testing.T) {
		app := newTestApp(t)
		app.seedVerifiedUser("weak-then-strong@test.local", "Password1", model.RoleUser)

		rec := app.request(http.MethodPost, "/v1/auth/forgot-password", map[string]any{
			"email": "weak-then-strong@test.local",
		}, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("forgot-password: got %d body %s, want 200", rec.Code, rec.Body.String())
		}
		token := extractResetToken(t, app.mail.last().BodyText)

		// A weak password is rejected before the token is touched.
		rec = app.request(http.MethodPost, "/v1/auth/reset-password", map[string]any{
			"token":        token,
			"new_password": "weak",
		}, "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("reset-password (weak, real token): got %d body %s, want 400", rec.Code, rec.Body.String())
		}

		// The same token still works with a strong password, confirming the
		// weak attempt did not consume it.
		rec = app.request(http.MethodPost, "/v1/auth/reset-password", map[string]any{
			"token":        token,
			"new_password": "StrongPass9",
		}, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("reset-password (strong, same token): got %d body %s, want 200", rec.Code, rec.Body.String())
		}
	})
}
