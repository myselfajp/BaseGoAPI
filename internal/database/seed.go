package database

import (
	"log"
	"strings"

	"github.com/myselfajp/BaseGoAPI/internal/config"
	"github.com/myselfajp/BaseGoAPI/internal/core/security"
	"github.com/myselfajp/BaseGoAPI/internal/model"
	"github.com/myselfajp/BaseGoAPI/internal/repository"
)

// SeedAdmin creates the initial administrator account when configured to do so
// and one does not already exist. It is the equivalent of app/db/seed.py.
func SeedAdmin(userRepo *repository.UserRepository, cfg *config.Config) error {
	if !cfg.CreateAdminOnStartup {
		return nil
	}

	email := strings.ToLower(strings.TrimSpace(cfg.AdminEmail))
	if email == "" {
		log.Println("[seed] CREATE_ADMIN_ON_STARTUP is enabled but ADMIN_EMAIL is empty; skipping")
		return nil
	}

	existing, err := userRepo.GetByEmail(email)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	hashed, err := security.HashPassword(cfg.AdminPassword)
	if err != nil {
		return err
	}

	admin := &model.User{
		Email:              email,
		PasswordHash:       hashed,
		Role:               model.RoleAdmin,
		FullName:           "System Administrator",
		PhoneNumber:        "",
		IsActive:           true,
		IsEmailVerified:    true, // the bootstrap admin is auto-verified
		IsTwoFactorEnabled: false,
	}
	if err := userRepo.Create(admin); err != nil {
		return err
	}

	log.Printf("[seed] Admin user created: %s", email)
	return nil
}
