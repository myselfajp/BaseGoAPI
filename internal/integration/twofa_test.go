package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/myselfajp/BaseGoAPI/internal/core/security"
	"github.com/myselfajp/BaseGoAPI/internal/model"
)

// twoFALoginBody captures the fields we assert on across both the OTP-challenge
// response (LoginChallengeResponse), the authenticated response (LoginResponse),
// and the error envelope ({"detail": ...}).
type twoFALoginBody struct {
	Status      string `json:"status"`
	AccessToken string `json:"access_token"`
	ChallengeID string `json:"challenge_id"`
	Detail      string `json:"detail"`
}

// initiate2FALogin performs the first login leg (password only) for a user that
// requires a second factor, asserts an OTP challenge was issued, and returns the
// challenge id together with the OTP code pulled from the delivered email.
func initiate2FALogin(t *testing.T, app *testApp, email, password string) (challengeID, otpCode string) {
	t.Helper()
	rec := app.request(http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    email,
		"password": password,
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("initiate 2FA login: got status %d body %s, want 200", rec.Code, rec.Body.String())
	}
	var resp twoFALoginBody
	decodeBody(t, rec, &resp)
	if resp.Status != "otp_required" {
		t.Fatalf("initiate 2FA login: got status %q, want %q (body %s)", resp.Status, "otp_required", rec.Body.String())
	}
	if resp.ChallengeID == "" {
		t.Fatalf("initiate 2FA login: expected non-empty challenge_id, body %s", rec.Body.String())
	}
	return resp.ChallengeID, extractOTP(t, app.mail.last().BodyText)
}

// TestLoginTwoFactorSeededUserFlow exercises the happy path for a user whose
// account has 2FA explicitly enabled: password -> OTP challenge -> OTP -> token.
func TestLoginTwoFactorSeededUserFlow(t *testing.T) {
	app := newTestApp(t)
	const email, password = "twofa@test.local", "Password1"
	app.seedUser(email, password, model.RoleUser, true, true, true)

	// Step 1: a correct password yields an OTP challenge, not a token.
	rec := app.request(http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    email,
		"password": password,
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login step 1: got %d body %s, want 200", rec.Code, rec.Body.String())
	}
	var challenge twoFALoginBody
	decodeBody(t, rec, &challenge)
	if challenge.Status != "otp_required" {
		t.Errorf("login step 1 status: got %q, want %q (body %s)", challenge.Status, "otp_required", rec.Body.String())
	}
	if challenge.ChallengeID == "" {
		t.Errorf("login step 1: expected non-empty challenge_id, body %s", rec.Body.String())
	}
	if challenge.AccessToken != "" {
		t.Errorf("login step 1: expected no access_token in challenge, got %q", challenge.AccessToken)
	}
	if app.mail.count() != 1 {
		t.Fatalf("login step 1: expected exactly 1 OTP email, got %d", app.mail.count())
	}

	// Step 2: submitting the correct OTP + challenge id completes authentication.
	otp := extractOTP(t, app.mail.last().BodyText)
	rec = app.request(http.MethodPost, "/v1/auth/login", map[string]any{
		"email":            email,
		"password":         password,
		"otp_code":         otp,
		"otp_challenge_id": challenge.ChallengeID,
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login step 2: got %d body %s, want 200", rec.Code, rec.Body.String())
	}
	var authed twoFALoginBody
	decodeBody(t, rec, &authed)
	if authed.Status != "authenticated" {
		t.Errorf("login step 2 status: got %q, want %q (body %s)", authed.Status, "authenticated", rec.Body.String())
	}
	if authed.AccessToken == "" {
		t.Errorf("login step 2: expected non-empty access_token, body %s", rec.Body.String())
	}
}

// TestLoginTwoFactorForceFlagFlow verifies that a user WITHOUT 2FA on their
// account is still forced through the OTP flow when ForceTwoFactorAuth is set.
func TestLoginTwoFactorForceFlagFlow(t *testing.T) {
	app := newTestApp(t)
	const email, password = "forced@test.local", "Password1"
	// Account 2FA is off...
	app.seedVerifiedUser(email, password, model.RoleUser)
	// ...but the global switch forces a second factor for everyone.
	app.cfg.ForceTwoFactorAuth = true

	challengeID, otp := initiate2FALogin(t, app, email, password)
	if app.mail.count() != 1 {
		t.Fatalf("force 2FA: expected exactly 1 OTP email, got %d", app.mail.count())
	}

	rec := app.request(http.MethodPost, "/v1/auth/login", map[string]any{
		"email":            email,
		"password":         password,
		"otp_code":         otp,
		"otp_challenge_id": challengeID,
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("force 2FA complete: got %d body %s, want 200", rec.Code, rec.Body.String())
	}
	var authed twoFALoginBody
	decodeBody(t, rec, &authed)
	if authed.Status != "authenticated" || authed.AccessToken == "" {
		t.Fatalf("force 2FA complete: unexpected response %+v (body %s)", authed, rec.Body.String())
	}
}

// TestLoginTwoFactorOTPErrors covers every rejection path in the OTP second leg:
// validation, wrong code, reuse, unknown/foreign challenge, and expiry.
func TestLoginTwoFactorOTPErrors(t *testing.T) {
	const email, password = "otperr@test.local", "Password1"

	// Cases that fail on the request shape alone, before any challenge lookup.
	t.Run("missing field validation", func(t *testing.T) {
		cases := []struct {
			name       string
			body       map[string]any
			wantDetail string
		}{
			{
				name:       "otp_code without otp_challenge_id",
				body:       map[string]any{"email": email, "password": password, "otp_code": "123456"},
				wantDetail: "otp_challenge_id is required when submitting an OTP code.",
			},
			{
				name:       "otp_challenge_id without otp_code",
				body:       map[string]any{"email": email, "password": password, "otp_challenge_id": "irrelevant-challenge-id"},
				wantDetail: "OTP code is required when submitting an otp_challenge_id.",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				app := newTestApp(t)
				app.seedUser(email, password, model.RoleUser, true, true, true)

				rec := app.request(http.MethodPost, "/v1/auth/login", tc.body, "")
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("got %d body %s, want 400", rec.Code, rec.Body.String())
				}
				var resp twoFALoginBody
				decodeBody(t, rec, &resp)
				if resp.Detail != tc.wantDetail {
					t.Errorf("detail: got %q, want %q (body %s)", resp.Detail, tc.wantDetail, rec.Body.String())
				}
				// This path short-circuits before any OTP is generated/sent.
				if app.mail.count() != 0 {
					t.Errorf("expected no OTP email, got %d", app.mail.count())
				}
			})
		}
	})

	// A syntactically valid but incorrect code is a 401.
	t.Run("wrong otp_code", func(t *testing.T) {
		app := newTestApp(t)
		app.seedUser(email, password, model.RoleUser, true, true, true)
		challengeID, otp := initiate2FALogin(t, app, email, password)

		rec := app.request(http.MethodPost, "/v1/auth/login", map[string]any{
			"email":            email,
			"password":         password,
			"otp_code":         otp + "0", // 7 digits can never equal the 6-digit OTP
			"otp_challenge_id": challengeID,
		}, "")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("got %d body %s, want 401", rec.Code, rec.Body.String())
		}
		var resp twoFALoginBody
		decodeBody(t, rec, &resp)
		if resp.Detail != "Incorrect OTP code." {
			t.Errorf("detail: got %q, want %q (body %s)", resp.Detail, "Incorrect OTP code.", rec.Body.String())
		}
	})

	// A challenge can only be redeemed once; the second attempt is a 400.
	t.Run("reused consumed challenge", func(t *testing.T) {
		app := newTestApp(t)
		app.seedUser(email, password, model.RoleUser, true, true, true)
		challengeID, otp := initiate2FALogin(t, app, email, password)

		body := map[string]any{
			"email":            email,
			"password":         password,
			"otp_code":         otp,
			"otp_challenge_id": challengeID,
		}
		// First submission succeeds and consumes the challenge.
		rec := app.request(http.MethodPost, "/v1/auth/login", body, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("first submission: got %d body %s, want 200", rec.Code, rec.Body.String())
		}
		// Reusing the now-consumed challenge is rejected.
		rec = app.request(http.MethodPost, "/v1/auth/login", body, "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("reuse: got %d body %s, want 400", rec.Code, rec.Body.String())
		}
		var resp twoFALoginBody
		decodeBody(t, rec, &resp)
		if resp.Detail != "OTP has already been used. Please request a new code." {
			t.Errorf("detail: got %q, want %q (body %s)", resp.Detail, "OTP has already been used. Please request a new code.", rec.Body.String())
		}
	})

	// A challenge id that does not exist at all is a 400.
	t.Run("nonexistent challenge id", func(t *testing.T) {
		app := newTestApp(t)
		app.seedUser(email, password, model.RoleUser, true, true, true)

		rec := app.request(http.MethodPost, "/v1/auth/login", map[string]any{
			"email":            email,
			"password":         password,
			"otp_code":         "123456",
			"otp_challenge_id": "11111111-1111-1111-1111-111111111111",
		}, "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("got %d body %s, want 400", rec.Code, rec.Body.String())
		}
		var resp twoFALoginBody
		decodeBody(t, rec, &resp)
		if resp.Detail != "Invalid OTP challenge. Please request a new code." {
			t.Errorf("detail: got %q, want %q (body %s)", resp.Detail, "Invalid OTP challenge. Please request a new code.", rec.Body.String())
		}
	})

	// A real challenge that belongs to a DIFFERENT user is also a 400, even with
	// that other user's correct code, because ownership is checked first.
	t.Run("foreign challenge id belongs to another user", func(t *testing.T) {
		app := newTestApp(t)
		app.seedUser(email, password, model.RoleUser, true, true, true)
		const otherEmail = "otherotp@test.local"
		app.seedUser(otherEmail, password, model.RoleUser, true, true, true)

		// The other user obtains a valid challenge + code...
		otherChallenge, otherOTP := initiate2FALogin(t, app, otherEmail, password)

		// ...which the first user tries (and fails) to redeem.
		rec := app.request(http.MethodPost, "/v1/auth/login", map[string]any{
			"email":            email,
			"password":         password,
			"otp_code":         otherOTP,
			"otp_challenge_id": otherChallenge,
		}, "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("got %d body %s, want 400", rec.Code, rec.Body.String())
		}
		var resp twoFALoginBody
		decodeBody(t, rec, &resp)
		if resp.Detail != "Invalid OTP challenge. Please request a new code." {
			t.Errorf("detail: got %q, want %q (body %s)", resp.Detail, "Invalid OTP challenge. Please request a new code.", rec.Body.String())
		}
	})

	// An expired challenge (inserted directly with a past ExpiresAt) is a 400.
	t.Run("expired otp", func(t *testing.T) {
		app := newTestApp(t)
		user := app.seedUser(email, password, model.RoleUser, true, true, true)

		const code = "135790"
		hash, err := security.HashPassword(code)
		if err != nil {
			t.Fatalf("hash otp: %v", err)
		}
		otp := &model.LoginOTP{
			UserID:    user.ID,
			CodeHash:  hash,
			ExpiresAt: time.Now().UTC().Add(-1 * time.Minute),
		}
		if err := app.db.Create(otp).Error; err != nil {
			t.Fatalf("insert expired otp: %v", err)
		}

		rec := app.request(http.MethodPost, "/v1/auth/login", map[string]any{
			"email":            email,
			"password":         password,
			"otp_code":         code,
			"otp_challenge_id": otp.ID,
		}, "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("got %d body %s, want 400", rec.Code, rec.Body.String())
		}
		var resp twoFALoginBody
		decodeBody(t, rec, &resp)
		if resp.Detail != "OTP has expired. Please request a new code." {
			t.Errorf("detail: got %q, want %q (body %s)", resp.Detail, "OTP has expired. Please request a new code.", rec.Body.String())
		}
	})
}
