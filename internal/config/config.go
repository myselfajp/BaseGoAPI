// Package config loads application settings from environment variables (and an
// optional .env file). It is the Go equivalent of app/core/config.py.
package config

import (
	"fmt"
	"log"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config holds every configurable value for the application. Each field is
// populated from the environment variable named in its `env` tag.
type Config struct {
	// Project configuration
	ProjectName string `env:"PROJECT_NAME"`
	APIV1Prefix string `env:"API_V1_PREFIX" envDefault:"/v1"`

	// JWT configuration
	JWTSecretKey             string `env:"JWT_SECRET_KEY"`
	JWTAlgorithm             string `env:"JWT_ALGORITHM" envDefault:"HS256"`
	AccessTokenExpireMinutes int    `env:"ACCESS_TOKEN_EXPIRE_MINUTES" envDefault:"1440"`

	// Database configuration. If DatabaseURL is empty it is built from the
	// individual Postgres components below.
	DatabaseURL      string `env:"DATABASE_URL"`
	PostgresHost     string `env:"POSTGRES_HOST"`
	PostgresPort     int    `env:"POSTGRES_PORT" envDefault:"5432"`
	PostgresDB       string `env:"POSTGRES_DB"`
	PostgresUser     string `env:"POSTGRES_USER"`
	PostgresPassword string `env:"POSTGRES_PASSWORD"`
	PostgresSSLMode  string `env:"POSTGRES_SSLMODE" envDefault:"disable"`

	// Admin bootstrap configuration
	CreateAdminOnStartup bool   `env:"CREATE_ADMIN_ON_STARTUP" envDefault:"true"`
	AdminEmail           string `env:"ADMIN_EMAIL"`
	AdminPassword        string `env:"ADMIN_PASSWORD"`

	// CORS configuration ("*" or a comma-separated list of origins)
	CORSOrigins string `env:"CORS_ORIGINS" envDefault:"*"`

	// Auth rate limiting: requests per minute per IP on the auth endpoints.
	// A value <= 0 disables rate limiting (useful in tests).
	AuthRateLimitPerMinute int `env:"AUTH_RATE_LIMIT_PER_MINUTE" envDefault:"5"`

	// Email provider configuration ("aws_ses" or "smtp")
	EmailProvider string `env:"EMAIL_PROVIDER" envDefault:"smtp"`
	SenderEmail   string `env:"SENDER_EMAIL"`

	// AWS SES configuration (required if EmailProvider == "aws_ses")
	AWSAccessKeyID     string `env:"AWS_ACCESS_KEY_ID"`
	AWSSecretAccessKey string `env:"AWS_SECRET_ACCESS_KEY"`
	AWSSESRegion       string `env:"AWS_SES_REGION"`

	// SMTP configuration (required if EmailProvider == "smtp")
	SMTPHost     string `env:"SMTP_HOST"`
	SMTPPort     int    `env:"SMTP_PORT" envDefault:"587"`
	SMTPUser     string `env:"SMTP_USER"`
	SMTPPassword string `env:"SMTP_PASSWORD"`
	SMTPUseTLS   bool   `env:"SMTP_USE_TLS" envDefault:"true"`
	SMTPUseSSL   bool   `env:"SMTP_USE_SSL" envDefault:"false"`

	// OTP configuration
	OTPLength            int `env:"OTP_LENGTH" envDefault:"6"`
	OTPExpirationMinutes int `env:"OTP_EXPIRATION_MINUTES" envDefault:"10"`

	// Email verification configuration
	EmailVerificationTokenExpirationMinutes int  `env:"EMAIL_VERIFICATION_TOKEN_EXPIRATION_MINUTES" envDefault:"60"`
	RequireEmailVerification                bool `env:"REQUIRE_EMAIL_VERIFICATION" envDefault:"true"`

	// Password reset configuration
	PasswordResetTokenExpirationMinutes int `env:"PASSWORD_RESET_TOKEN_EXPIRATION_MINUTES" envDefault:"60"`

	// Two-factor authentication configuration
	ForceTwoFactorAuth bool `env:"FORCE_TWO_FACTOR_AUTH" envDefault:"false"`

	// Site configuration
	BaseURL string `env:"BASE_URL" envDefault:"http://localhost:8000"`

	// HTTP server configuration
	ServerPort string `env:"SERVER_PORT" envDefault:"8000"`
}

// Load reads the .env file (if present) and parses the environment into a
// Config. It exits the process on failure, mirroring the fail-fast behaviour of
// the FastAPI settings loader.
func Load() *Config {
	// Loading .env is best-effort: in containerised deployments the values are
	// usually injected directly into the environment.
	if err := godotenv.Load(); err != nil {
		log.Printf("no .env file loaded (%v); relying on process environment", err)
	}

	cfg, err := Parse()
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}
	return cfg
}

// Parse builds a Config from the current process environment. It does not read
// the .env file and returns an error instead of exiting, which makes it
// straightforward to unit test.
func Parse() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = cfg.buildDatabaseURL()
		log.Println("DATABASE_URL was automatically constructed from database components")
	}

	return cfg, nil
}

// buildDatabaseURL assembles a Postgres DSN from the individual components.
func (c *Config) buildDatabaseURL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.PostgresUser,
		c.PostgresPassword,
		c.PostgresHost,
		c.PostgresPort,
		c.PostgresDB,
		c.PostgresSSLMode,
	)
}
