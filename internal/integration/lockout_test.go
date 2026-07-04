package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/myselfajp/BaseGoAPI/internal/model"
)

// lockoutThreshold mirrors the (unexported) maxFailedLoginAttempts constant in
// internal/service/auth.go: after this many failed attempts a non-admin account
// is locked.
const lockoutThreshold = 3

// lockoutLogin posts a login request with the given credentials.
func lockoutLogin(app *testApp, email, password string) *httptest.ResponseRecorder {
	return app.request(http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    email,
		"password": password,
	}, "")
}

// lockoutReload fetches the current persisted state of a user from the database.
func lockoutReload(t *testing.T, app *testApp, email string) *model.User {
	t.Helper()
	u, err := app.userRepo.GetByEmail(email)
	if err != nil {
		t.Fatalf("reload user %q: %v", email, err)
	}
	if u == nil {
		t.Fatalf("reload user %q: not found", email)
	}
	return u
}

// lockoutDetail extracts the {"detail": ...} message from an error response.
func lockoutDetail(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Detail string `json:"detail"`
	}
	decodeBody(t, rec, &body)
	return body.Detail
}

// TestLoginLockoutNonAdminProgression walks a verified non-admin account through
// wrong-password attempts 1 and 2 (401 with remaining-attempt messages) and then
// attempt 3 (403 lock), asserting the persisted counter and active flag at each
// step.
func TestLoginLockoutNonAdminProgression(t *testing.T) {
	app := newTestApp(t)
	const email = "locktarget@test.local"
	const password = "Password1"
	app.seedVerifiedUser(email, password, model.RoleUser)

	steps := []struct {
		name         string
		wantCode     int
		wantDetail   string
		wantAttempts int
		wantActive   bool
	}{
		{
			name:         "first wrong attempt leaves 2 remaining",
			wantCode:     http.StatusUnauthorized,
			wantDetail:   "Incorrect credentials. 2 attempt(s) remaining before account lock.",
			wantAttempts: 1,
			wantActive:   true,
		},
		{
			name:         "second wrong attempt leaves 1 remaining",
			wantCode:     http.StatusUnauthorized,
			wantDetail:   "Incorrect credentials. 1 attempt(s) remaining before account lock.",
			wantAttempts: 2,
			wantActive:   true,
		},
		{
			name:         "third wrong attempt locks the account",
			wantCode:     http.StatusForbidden,
			wantDetail:   "Your account has been locked due to multiple failed login attempts. Please contact an administrator to unlock your account.",
			wantAttempts: 0, // reset to 0 when the lock is applied
			wantActive:   false,
		},
	}

	for _, st := range steps {
		t.Run(st.name, func(t *testing.T) {
			rec := lockoutLogin(app, email, "wrong-password")
			if rec.Code != st.wantCode {
				t.Fatalf("login: got status %d, want %d (body %s)", rec.Code, st.wantCode, rec.Body.String())
			}
			if got := lockoutDetail(t, rec); got != st.wantDetail {
				t.Fatalf("login: got detail %q, want %q (body %s)", got, st.wantDetail, rec.Body.String())
			}
			u := lockoutReload(t, app, email)
			if u.FailedLoginAttempts != st.wantAttempts {
				t.Errorf("failed_login_attempts: got %d, want %d", u.FailedLoginAttempts, st.wantAttempts)
			}
			if u.IsActive != st.wantActive {
				t.Errorf("is_active: got %v, want %v", u.IsActive, st.wantActive)
			}
		})
	}
}

// TestLoginCorrectPasswordAfterLockoutIsForbidden locks a non-admin account and
// then confirms that even the CORRECT password is rejected with 403 while the
// account is inactive.
func TestLoginCorrectPasswordAfterLockoutIsForbidden(t *testing.T) {
	app := newTestApp(t)
	const email = "postlock@test.local"
	const password = "Password1"
	app.seedVerifiedUser(email, password, model.RoleUser)

	// Drive the account into a locked state with wrong passwords.
	for i := 0; i < lockoutThreshold; i++ {
		rec := lockoutLogin(app, email, "wrong-password")
		wantCode := http.StatusUnauthorized
		if i == lockoutThreshold-1 {
			wantCode = http.StatusForbidden
		}
		if rec.Code != wantCode {
			t.Fatalf("wrong attempt %d: got status %d, want %d (body %s)", i+1, rec.Code, wantCode, rec.Body.String())
		}
	}
	if u := lockoutReload(t, app, email); u.IsActive {
		t.Fatalf("account should be inactive after lockout, is_active=%v", u.IsActive)
	}

	// The correct password must still be rejected while the account is inactive.
	rec := lockoutLogin(app, email, password)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("correct password after lockout: got status %d, want %d (body %s)",
			rec.Code, http.StatusForbidden, rec.Body.String())
	}
	const wantDetail = "Your account has been locked. Please contact an administrator to unlock your account."
	if got := lockoutDetail(t, rec); got != wantDetail {
		t.Fatalf("correct password after lockout: got detail %q, want %q (body %s)", got, wantDetail, rec.Body.String())
	}
	if u := lockoutReload(t, app, email); u.IsActive {
		t.Fatalf("account should remain inactive after a correct-password attempt, is_active=%v", u.IsActive)
	}
}

// TestLoginAdminNeverLocksOut confirms an admin account survives many wrong
// passwords: every attempt is a plain 401 and the account stays active with a
// zero failure counter (admins are exempt from the lockout policy).
func TestLoginAdminNeverLocksOut(t *testing.T) {
	app := newTestApp(t)
	const email = "admin-lock@test.local"
	const password = "Password1"
	app.seedVerifiedUser(email, password, model.RoleAdmin)

	attempts := lockoutThreshold + 2 // well past the non-admin threshold
	for i := 0; i < attempts; i++ {
		rec := lockoutLogin(app, email, "wrong-password")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("admin wrong attempt %d: got status %d, want %d (body %s)",
				i+1, rec.Code, http.StatusUnauthorized, rec.Body.String())
		}
		if got := lockoutDetail(t, rec); got != "Incorrect credentials" {
			t.Errorf("admin wrong attempt %d: got detail %q, want %q", i+1, got, "Incorrect credentials")
		}
		u := lockoutReload(t, app, email)
		if !u.IsActive {
			t.Fatalf("admin account must stay active after attempt %d, is_active=%v", i+1, u.IsActive)
		}
		if u.FailedLoginAttempts != 0 {
			t.Errorf("admin failed_login_attempts should stay 0, got %d after attempt %d", u.FailedLoginAttempts, i+1)
		}
	}
}

// TestLoginFailedCounterResetsOnSuccessfulLogin proves the failure counter is
// cleared by a successful login: 2 wrong attempts, then a correct login (200,
// counter -> 0), then 2 more wrong attempts that only reach "1 remaining"
// instead of locking — which could only happen if the counter had reset.
func TestLoginFailedCounterResetsOnSuccessfulLogin(t *testing.T) {
	app := newTestApp(t)
	const email = "reset-counter@test.local"
	const password = "Password1"
	app.seedVerifiedUser(email, password, model.RoleUser)

	// Two wrong passwords: still below the lock threshold.
	for i := 0; i < 2; i++ {
		rec := lockoutLogin(app, email, "wrong-password")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("pre-reset wrong attempt %d: got status %d, want %d (body %s)",
				i+1, rec.Code, http.StatusUnauthorized, rec.Body.String())
		}
	}
	if u := lockoutReload(t, app, email); u.FailedLoginAttempts != 2 {
		t.Fatalf("failed_login_attempts before success: got %d, want 2", u.FailedLoginAttempts)
	}

	// A correct password authenticates and must reset the counter.
	rec := lockoutLogin(app, email, password)
	if rec.Code != http.StatusOK {
		t.Fatalf("correct password: got status %d, want %d (body %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	var resp struct {
		Status      string `json:"status"`
		AccessToken string `json:"access_token"`
	}
	decodeBody(t, rec, &resp)
	if resp.Status != "authenticated" || resp.AccessToken == "" {
		t.Fatalf("unexpected login response: %+v (body %s)", resp, rec.Body.String())
	}
	if u := lockoutReload(t, app, email); u.FailedLoginAttempts != 0 {
		t.Fatalf("failed_login_attempts after success: got %d, want 0", u.FailedLoginAttempts)
	}

	// Two more wrong passwords must NOT immediately lock the account: if the
	// counter had not reset, the first of these would be the 3rd cumulative
	// failure and would return 403. Instead we expect the fresh 2-then-1
	// remaining progression, still 401.
	wantDetails := []string{
		"Incorrect credentials. 2 attempt(s) remaining before account lock.",
		"Incorrect credentials. 1 attempt(s) remaining before account lock.",
	}
	for i, want := range wantDetails {
		rec := lockoutLogin(app, email, "wrong-password")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("post-reset wrong attempt %d: got status %d, want %d (body %s)",
				i+1, rec.Code, http.StatusUnauthorized, rec.Body.String())
		}
		if got := lockoutDetail(t, rec); got != want {
			t.Errorf("post-reset wrong attempt %d: got detail %q, want %q", i+1, got, want)
		}
	}
	if u := lockoutReload(t, app, email); u.FailedLoginAttempts != 2 || !u.IsActive {
		t.Fatalf("after reset + 2 wrong: failed_login_attempts=%d is_active=%v, want 2 and true",
			u.FailedLoginAttempts, u.IsActive)
	}
}
