// Package integration contains black-box API tests that exercise the full HTTP
// stack (router -> middleware -> handler -> service -> repository) against a
// real PostgreSQL instance started via embedded-postgres — no Docker required.
//
// A single Postgres server is started once for the whole package (TestMain),
// the real SQL migrations are applied, and every test gets a clean slate via
// truncation. Tests must NOT call t.Parallel(): they share one database.
//
// Harness API for test authors:
//
//	app := newTestApp(t)                         // fresh wiring + clean tables
//	app.cfg.ForceTwoFactorAuth = true            // tweak config before requests
//	rec := app.request(method, path, body, token) // body is any JSON-marshalable value (or nil)
//	decodeBody(t, rec, &target)                  // unmarshal JSON response
//	u := app.seedVerifiedUser(email, pw, role)   // insert a ready-to-login user
//	tok := app.tokenFor(email)                   // mint a JWT for that user
//	app.mail.last().BodyText                      // inspect the last "sent" email
//	tok := extractVerifyToken(t, app.mail.last().BodyText)
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/myselfajp/BaseGoAPI/internal/config"
	"github.com/myselfajp/BaseGoAPI/internal/core/jwtutil"
	"github.com/myselfajp/BaseGoAPI/internal/core/security"
	"github.com/myselfajp/BaseGoAPI/internal/database"
	"github.com/myselfajp/BaseGoAPI/internal/handler"
	"github.com/myselfajp/BaseGoAPI/internal/model"
	"github.com/myselfajp/BaseGoAPI/internal/repository"
	"github.com/myselfajp/BaseGoAPI/internal/router"
	"github.com/myselfajp/BaseGoAPI/internal/service"
)

const testDSN = "postgres://postgres:postgres@localhost:54329/testdb?sslmode=disable"

// testDB is the shared connection to the embedded Postgres server.
var testDB *gorm.DB

// TestMain boots one embedded Postgres server, migrates it, runs the suite,
// then tears everything down.
func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	gin.DefaultWriter = io.Discard

	pg := embeddedpostgres.NewDatabase(
		embeddedpostgres.DefaultConfig().
			Username("postgres").
			Password("postgres").
			Database("testdb").
			Port(54329).
			Version(embeddedpostgres.V15),
	)
	if err := pg.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start embedded postgres: %v\n", err)
		os.Exit(1)
	}

	code := func() int {
		defer func() { _ = pg.Stop() }()

		db, err := database.Connect(baseConfig())
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to connect: %v\n", err)
			return 1
		}
		if err := database.Migrate(db); err != nil {
			fmt.Fprintf(os.Stderr, "failed to migrate: %v\n", err)
			return 1
		}
		testDB = db
		return m.Run()
	}()

	os.Exit(code)
}

// baseConfig returns a fresh test configuration. A new copy is handed to each
// test so it can mutate feature flags without affecting others.
func baseConfig() *config.Config {
	return &config.Config{
		ProjectName:                             "Test App",
		APIV1Prefix:                             "/v1",
		JWTSecretKey:                            "test-secret-key-please-change",
		JWTAlgorithm:                            "HS256",
		AccessTokenExpireMinutes:                60,
		DatabaseURL:                             testDSN,
		CORSOrigins:                             "*",
		AuthRateLimitPerMinute:                  0, // disabled for tests
		EmailProvider:                           "smtp",
		SenderEmail:                             "noreply@test.local",
		OTPLength:                               6,
		OTPExpirationMinutes:                    10,
		EmailVerificationTokenExpirationMinutes: 60,
		RequireEmailVerification:                true,
		PasswordResetTokenExpirationMinutes:     60,
		ForceTwoFactorAuth:                      false,
		BaseURL:                                 "http://localhost:8000",
		CreateAdminOnStartup:                    false,
		ServerPort:                              "8000",
	}
}

// --- Fake email sender ---

type sentEmail struct {
	Recipient string
	Subject   string
	BodyText  string
	BodyHTML  string
}

// fakeSender implements email.Sender and records everything "sent" so tests can
// pull verification tokens / OTP codes out of the message body.
type fakeSender struct {
	mu       sync.Mutex
	sent     []sentEmail
	failNext bool // when true, the next Send fails once (to test rollback paths)
}

func (f *fakeSender) Send(recipient, subject, bodyText, bodyHTML string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNext {
		f.failNext = false
		return fmt.Errorf("simulated email delivery failure")
	}
	f.sent = append(f.sent, sentEmail{recipient, subject, bodyText, bodyHTML})
	return nil
}

func (f *fakeSender) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}

func (f *fakeSender) last() sentEmail {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sent) == 0 {
		return sentEmail{}
	}
	return f.sent[len(f.sent)-1]
}

// --- Test application wiring ---

type testApp struct {
	t        *testing.T
	router   *gin.Engine
	db       *gorm.DB
	cfg      *config.Config
	mail     *fakeSender
	userRepo *repository.UserRepository
}

// newTestApp truncates all tables and wires a fresh application graph against
// the shared database.
func newTestApp(t *testing.T) *testApp {
	t.Helper()
	truncateAll(t)

	cfg := baseConfig()
	mail := &fakeSender{}

	userRepo := repository.NewUserRepository(testDB)
	otpRepo := repository.NewLoginOTPRepository(testDB)
	evtRepo := repository.NewEmailVerificationTokenRepository(testDB)
	prtRepo := repository.NewPasswordResetTokenRepository(testDB)

	userService := service.NewUserService(userRepo)
	authService := service.NewAuthService(cfg, userRepo, otpRepo, mail)
	evService := service.NewEmailVerificationService(cfg, evtRepo, userService)
	prService := service.NewPasswordResetService(cfg, prtRepo, userRepo)

	authHandler := handler.NewAuthHandler(cfg, userService, authService, evService, prService, mail)
	userHandler := handler.NewUserHandler(userService)

	r := router.Setup(cfg, userRepo, authHandler, userHandler)

	return &testApp{t: t, router: r, db: testDB, cfg: cfg, mail: mail, userRepo: userRepo}
}

func truncateAll(t *testing.T) {
	t.Helper()
	err := testDB.Exec(
		`TRUNCATE users, email_verification_tokens, login_otps, password_reset_tokens RESTART IDENTITY CASCADE`,
	).Error
	if err != nil {
		t.Fatalf("failed to truncate tables: %v", err)
	}
}

// request performs an HTTP request against the router. body may be nil or any
// JSON-marshalable value; token, when non-empty, is sent as a Bearer token.
func (a *testApp) request(method, path string, body any, token string) *httptest.ResponseRecorder {
	a.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			a.t.Fatalf("failed to marshal request body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)
	return rec
}

// decodeBody unmarshals a JSON response body into target.
func decodeBody(t *testing.T, rec *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), target); err != nil {
		t.Fatalf("failed to decode response body %q: %v", rec.Body.String(), err)
	}
}

// seedUser inserts a user directly with full control over its flags.
func (a *testApp) seedUser(email, password, role string, verified, twoFA, active bool) *model.User {
	a.t.Helper()
	hash, err := security.HashPassword(password)
	if err != nil {
		a.t.Fatalf("failed to hash password: %v", err)
	}
	u := &model.User{
		Email:              strings.ToLower(email),
		PasswordHash:       hash,
		Role:               role,
		FullName:           "Test User",
		PhoneNumber:        "+1234567890",
		IsActive:           active,
		IsEmailVerified:    verified,
		IsTwoFactorEnabled: twoFA,
	}
	if err := a.userRepo.Create(u); err != nil {
		a.t.Fatalf("failed to seed user: %v", err)
	}
	return u
}

// seedVerifiedUser inserts an active, email-verified user without 2FA.
func (a *testApp) seedVerifiedUser(email, password, role string) *model.User {
	return a.seedUser(email, password, role, true, false, true)
}

// tokenFor mints a JWT access token for the given email.
func (a *testApp) tokenFor(email string) string {
	a.t.Helper()
	token, err := jwtutil.CreateAccessToken(a.cfg, email, 0)
	if err != nil {
		a.t.Fatalf("failed to create token: %v", err)
	}
	return token
}

// seedAdminWithToken seeds a verified admin and returns it with a valid token.
func (a *testApp) seedAdminWithToken(email, password string) (*model.User, string) {
	u := a.seedVerifiedUser(email, password, model.RoleAdmin)
	return u, a.tokenFor(email)
}

// --- Email body extractors ---

var (
	verifyTokenRe = regexp.MustCompile(`verify-email\?token=([^\s"&<]+)`)
	resetTokenRe  = regexp.MustCompile(`reset-password\?token=([^\s"&<]+)`)
	otpRe         = regexp.MustCompile(`login code is:\s*([0-9]+)`)
)

func extractVerifyToken(t *testing.T, body string) string {
	t.Helper()
	return extract(t, verifyTokenRe, body, "verification token")
}

func extractResetToken(t *testing.T, body string) string {
	t.Helper()
	return extract(t, resetTokenRe, body, "reset token")
}

func extractOTP(t *testing.T, body string) string {
	t.Helper()
	return extract(t, otpRe, body, "OTP code")
}

func extract(t *testing.T, re *regexp.Regexp, body, what string) string {
	t.Helper()
	m := re.FindStringSubmatch(body)
	if len(m) < 2 {
		t.Fatalf("no %s found in email body: %s", what, body)
	}
	return m[1]
}

// --- Smoke tests validating the harness itself ---

func TestHarnessBoots(t *testing.T) {
	app := newTestApp(t)

	rec := app.request(http.MethodGet, "/v1/health", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("health check: status %d body %s", rec.Code, rec.Body.String())
	}

	app.seedVerifiedUser("smoke@test.local", "Password1", model.RoleUser)
	var got model.User
	if err := app.db.Where("email = ?", "smoke@test.local").First(&got).Error; err != nil {
		t.Fatalf("failed to read seeded user: %v", err)
	}
	if got.ID == 0 || !got.IsEmailVerified {
		t.Fatalf("unexpected seeded user: %+v", got)
	}
}

func TestHarnessRegisterVerifyLoginFlow(t *testing.T) {
	app := newTestApp(t)

	rec := app.request(http.MethodPost, "/v1/auth/register", map[string]any{
		"email":        "alice@test.local",
		"password":     "Password1",
		"full_name":    "Alice",
		"phone_number": "+1234567890",
	}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: got %d body %s", rec.Code, rec.Body.String())
	}
	if app.mail.count() != 1 {
		t.Fatalf("expected 1 verification email, got %d", app.mail.count())
	}

	// Login before verification is rejected.
	rec = app.request(http.MethodPost, "/v1/auth/login", map[string]any{
		"email": "alice@test.local", "password": "Password1",
	}, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("login before verify: expected 403, got %d body %s", rec.Code, rec.Body.String())
	}

	// Verify using the token from the captured email.
	token := extractVerifyToken(t, app.mail.last().BodyText)
	rec = app.request(http.MethodPost, "/v1/auth/verify-email?token="+token, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("verify-email: got %d body %s", rec.Code, rec.Body.String())
	}

	// Login now succeeds with a JWT.
	rec = app.request(http.MethodPost, "/v1/auth/login", map[string]any{
		"email": "alice@test.local", "password": "Password1",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login after verify: got %d body %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Status      string `json:"status"`
		AccessToken string `json:"access_token"`
	}
	decodeBody(t, rec, &resp)
	if resp.Status != "authenticated" || resp.AccessToken == "" {
		t.Fatalf("unexpected login response: %+v", resp)
	}
}
