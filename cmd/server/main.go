// Command server is the application entry point. It wires configuration, the
// database, repositories, services and handlers together and starts the HTTP
// server. It is the Go equivalent of app/main.py.
package main

import (
	"log"

	"github.com/myselfajp/BaseGoAPI/internal/config"
	"github.com/myselfajp/BaseGoAPI/internal/core/email"
	"github.com/myselfajp/BaseGoAPI/internal/database"
	"github.com/myselfajp/BaseGoAPI/internal/handler"
	"github.com/myselfajp/BaseGoAPI/internal/repository"
	"github.com/myselfajp/BaseGoAPI/internal/router"
	"github.com/myselfajp/BaseGoAPI/internal/service"
)

func main() {
	// Configuration
	cfg := config.Load()

	// Database: connect, migrate, seed.
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		log.Fatalf("database migration failed: %v", err)
	}

	// Repositories
	userRepo := repository.NewUserRepository(db)
	loginOTPRepo := repository.NewLoginOTPRepository(db)
	emailVerifyRepo := repository.NewEmailVerificationTokenRepository(db)
	passwordResetRepo := repository.NewPasswordResetTokenRepository(db)

	if err := database.SeedAdmin(userRepo, cfg); err != nil {
		log.Fatalf("admin seeding failed: %v", err)
	}

	// Core services
	emailSender := email.NewService(cfg)

	// Business services
	userService := service.NewUserService(userRepo)
	authService := service.NewAuthService(cfg, userRepo, loginOTPRepo, emailSender)
	emailVerifyService := service.NewEmailVerificationService(cfg, emailVerifyRepo, userService)
	passwordResetService := service.NewPasswordResetService(cfg, passwordResetRepo, userRepo)

	// HTTP handlers
	authHandler := handler.NewAuthHandler(cfg, userService, authService, emailVerifyService, passwordResetService, emailSender)
	userHandler := handler.NewUserHandler(userService)

	// Router
	engine := router.Setup(cfg, userRepo, authHandler, userHandler)

	addr := ":" + cfg.ServerPort
	log.Printf("%s starting on %s", cfg.ProjectName, addr)
	if err := engine.Run(addr); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
