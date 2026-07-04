package service

import (
	"strings"
	"testing"

	"github.com/myselfajp/BaseGoAPI/internal/config"
)

// emailTestConfig returns a Config with deterministic values so the rendered
// email content can be asserted exactly.
func emailTestConfig() *config.Config {
	return &config.Config{
		ProjectName:                             "TestProject",
		BaseURL:                                 "https://api.example.com",
		EmailVerificationTokenExpirationMinutes: 45,
		PasswordResetTokenExpirationMinutes:     30,
	}
}

// assertASCII fails if s contains any non-ASCII rune, which would indicate a
// leftover foreign-language (non-English) string.
func assertASCII(t *testing.T, label, s string) {
	t.Helper()
	for i, r := range s {
		if r > 127 {
			t.Errorf("%s contains non-ASCII rune %q at byte offset %d; content is not generic English: %q", label, r, i, s)
			return
		}
	}
}

func TestBuildVerificationEmail(t *testing.T) {
	cfg := emailTestConfig()
	const token = "verify-token-abc123"
	email := BuildVerificationEmail(cfg, "Ada Lovelace", token)

	wantURL := cfg.BaseURL + "/v1/auth/verify-email?token=" + token

	if email.Subject == "" {
		t.Fatalf("Subject = %q, want non-empty", email.Subject)
	}

	bodies := []struct {
		name string
		body string
	}{
		{"BodyText", email.BodyText},
		{"BodyHTML", email.BodyHTML},
	}
	for _, b := range bodies {
		t.Run(b.name, func(t *testing.T) {
			if !strings.Contains(b.body, wantURL) {
				t.Errorf("%s does not contain verify-email URL\n got: %q\nwant substring: %q", b.name, b.body, wantURL)
			}
			if !strings.Contains(b.body, cfg.BaseURL) {
				t.Errorf("%s does not contain BaseURL %q\n got: %q", b.name, cfg.BaseURL, b.body)
			}
			if !strings.Contains(b.body, token) {
				t.Errorf("%s does not contain token %q\n got: %q", b.name, token, b.body)
			}
			if !strings.Contains(b.body, cfg.ProjectName) {
				t.Errorf("%s does not contain ProjectName %q\n got: %q", b.name, cfg.ProjectName, b.body)
			}
		})
	}
}

func TestBuildPasswordResetEmail(t *testing.T) {
	cfg := emailTestConfig()
	const token = "reset-token-xyz789"
	email := BuildPasswordResetEmail(cfg, "Grace Hopper", token)

	wantURL := cfg.BaseURL + "/v1/auth/reset-password?token=" + token
	// %d minutes is rendered from PasswordResetTokenExpirationMinutes.
	wantExpiry := "30 minutes"

	if email.Subject == "" {
		t.Fatalf("Subject = %q, want non-empty", email.Subject)
	}

	bodies := []struct {
		name string
		body string
	}{
		{"BodyText", email.BodyText},
		{"BodyHTML", email.BodyHTML},
	}
	for _, b := range bodies {
		t.Run(b.name, func(t *testing.T) {
			if !strings.Contains(b.body, wantURL) {
				t.Errorf("%s does not contain reset-password URL\n got: %q\nwant substring: %q", b.name, b.body, wantURL)
			}
			if !strings.Contains(b.body, token) {
				t.Errorf("%s does not contain token %q\n got: %q", b.name, token, b.body)
			}
			if !strings.Contains(b.body, wantExpiry) {
				t.Errorf("%s does not contain expiry %q\n got: %q", b.name, wantExpiry, b.body)
			}
		})
	}
}

func TestBuildOTPEmail(t *testing.T) {
	cfg := emailTestConfig()
	const otpCode = "483920"
	const expiresMinutes = 10
	email := BuildOTPEmail(cfg, "Katherine Johnson", otpCode, expiresMinutes)

	if email.Subject == "" {
		t.Fatalf("Subject = %q, want non-empty", email.Subject)
	}

	if !strings.Contains(email.BodyText, otpCode) {
		t.Errorf("BodyText does not contain OTP code %q\n got: %q", otpCode, email.BodyText)
	}
	// %d minutes is rendered from the expiresMinutes argument.
	if wantExpiry := "10 minutes"; !strings.Contains(email.BodyText, wantExpiry) {
		t.Errorf("BodyText does not contain expiry %q\n got: %q", wantExpiry, email.BodyText)
	}

	// The content must be generic English with no leftover foreign-language
	// strings. Assert every rendered string is ASCII and contains the expected
	// English phrasing.
	t.Run("GenericEnglish", func(t *testing.T) {
		assertASCII(t, "Subject", email.Subject)
		assertASCII(t, "BodyText", email.BodyText)
		assertASCII(t, "BodyHTML", email.BodyHTML)

		for _, phrase := range []string{"one-time login code", "did not request this code"} {
			if !strings.Contains(email.BodyText, phrase) {
				t.Errorf("BodyText missing expected English phrase %q\n got: %q", phrase, email.BodyText)
			}
		}
	})
}
