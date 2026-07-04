package config

import "testing"

// TestParseBuildsDatabaseURLFromComponents verifies that when DATABASE_URL is
// unset, Parse assembles the DSN from the individual POSTGRES_* components.
func TestParseBuildsDatabaseURLFromComponents(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "db.example.com")
	t.Setenv("POSTGRES_PORT", "6543")
	t.Setenv("POSTGRES_DB", "appdb")
	t.Setenv("POSTGRES_USER", "appuser")
	t.Setenv("POSTGRES_PASSWORD", "s3cret")
	t.Setenv("POSTGRES_SSLMODE", "require")
	// DATABASE_URL intentionally left unset so Parse must build it.

	cfg, err := Parse()
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	const want = "postgres://appuser:s3cret@db.example.com:6543/appdb?sslmode=require"
	if cfg.DatabaseURL != want {
		t.Fatalf("Parse() DatabaseURL = %q, want %q", cfg.DatabaseURL, want)
	}
}

// TestParseKeepsExplicitDatabaseURL verifies that an explicitly provided
// DATABASE_URL is preserved and NOT overwritten by the POSTGRES_* components.
func TestParseKeepsExplicitDatabaseURL(t *testing.T) {
	const explicit = "postgres://custom:pw@customhost:1234/mydb?sslmode=verify-full"
	t.Setenv("DATABASE_URL", explicit)
	// Set conflicting components to prove they are ignored when DATABASE_URL is set.
	t.Setenv("POSTGRES_HOST", "ignored-host")
	t.Setenv("POSTGRES_PORT", "9999")
	t.Setenv("POSTGRES_DB", "ignored-db")
	t.Setenv("POSTGRES_USER", "ignored-user")
	t.Setenv("POSTGRES_PASSWORD", "ignored-pw")
	t.Setenv("POSTGRES_SSLMODE", "disable")

	cfg, err := Parse()
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if cfg.DatabaseURL != explicit {
		t.Fatalf("Parse() DatabaseURL = %q, want unchanged %q", cfg.DatabaseURL, explicit)
	}
}

// TestParseAppliesDefaults verifies the documented default values apply when the
// corresponding environment variables are unset.
func TestParseAppliesDefaults(t *testing.T) {
	cfg, err := Parse()
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if cfg.AuthRateLimitPerMinute != 5 {
		t.Errorf("Parse() AuthRateLimitPerMinute = %d, want 5", cfg.AuthRateLimitPerMinute)
	}
	if cfg.APIV1Prefix != "/v1" {
		t.Errorf("Parse() APIV1Prefix = %q, want %q", cfg.APIV1Prefix, "/v1")
	}
}

// TestBuildDatabaseURLFormat verifies buildDatabaseURL formats a Postgres DSN
// from a known Config without touching the environment.
func TestBuildDatabaseURLFormat(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "require sslmode",
			cfg: Config{
				PostgresUser:     "appuser",
				PostgresPassword: "s3cret",
				PostgresHost:     "db.example.com",
				PostgresPort:     6543,
				PostgresDB:       "appdb",
				PostgresSSLMode:  "require",
			},
			want: "postgres://appuser:s3cret@db.example.com:6543/appdb?sslmode=require",
		},
		{
			name: "disable sslmode default port",
			cfg: Config{
				PostgresUser:     "postgres",
				PostgresPassword: "postgres",
				PostgresHost:     "localhost",
				PostgresPort:     5432,
				PostgresDB:       "base",
				PostgresSSLMode:  "disable",
			},
			want: "postgres://postgres:postgres@localhost:5432/base?sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.buildDatabaseURL()
			if got != tt.want {
				t.Fatalf("buildDatabaseURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
