package integration

import (
	"net/http"
	"testing"

	"github.com/myselfajp/BaseGoAPI/internal/database"
)

func TestSeedAdminCreatesVerifiedAdminOnce(t *testing.T) {
	app := newTestApp(t)
	app.cfg.CreateAdminOnStartup = true
	app.cfg.AdminEmail = "Boot.Admin@Test.Local" // mixed case: must be normalised
	app.cfg.AdminPassword = "BootAdmin1"

	// Seeding twice must create exactly one admin (idempotent startup).
	if err := database.SeedAdmin(app.userRepo, app.cfg); err != nil {
		t.Fatalf("first SeedAdmin: %v", err)
	}
	if err := database.SeedAdmin(app.userRepo, app.cfg); err != nil {
		t.Fatalf("second SeedAdmin: %v", err)
	}
	count, err := app.userRepo.CountAdmins()
	if err != nil {
		t.Fatalf("CountAdmins: %v", err)
	}
	if count != 1 {
		t.Fatalf("admin count = %d, want 1", count)
	}

	admin, err := app.userRepo.GetByEmail("boot.admin@test.local")
	if err != nil || admin == nil {
		t.Fatalf("seeded admin not found under normalised email: %v", err)
	}
	if !admin.IsEmailVerified || !admin.IsActive {
		t.Fatalf("seeded admin should be verified and active: %+v", admin)
	}

	// The seeded credentials actually work against the login endpoint.
	rec := app.request(http.MethodPost, "/v1/auth/login", map[string]any{
		"email": "boot.admin@test.local", "password": "BootAdmin1",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("seeded admin login: got %d body %s", rec.Code, rec.Body.String())
	}
}

func TestSeedAdminDisabledByFlag(t *testing.T) {
	app := newTestApp(t)
	app.cfg.CreateAdminOnStartup = false
	app.cfg.AdminEmail = "noboot@test.local"
	app.cfg.AdminPassword = "BootAdmin1"

	if err := database.SeedAdmin(app.userRepo, app.cfg); err != nil {
		t.Fatalf("SeedAdmin: %v", err)
	}
	count, err := app.userRepo.CountAdmins()
	if err != nil {
		t.Fatalf("CountAdmins: %v", err)
	}
	if count != 0 {
		t.Fatalf("admin count = %d, want 0 when seeding is disabled", count)
	}
}
