package jwtutil

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/myselfajp/BaseGoAPI/internal/config"
)

// newJWTTestConfig builds a *config.Config populated with just the fields that
// jwtutil reads. AccessTokenExpireMinutes mirrors the production default so that
// the "expiresMinutes <= 0 uses the default" branch produces a live token.
func newJWTTestConfig(secret, algorithm string) *config.Config {
	return &config.Config{
		JWTSecretKey:             secret,
		JWTAlgorithm:             algorithm,
		AccessTokenExpireMinutes: 1440,
	}
}

// tokenExpiry parses tokenString with cfg's secret and returns its ExpiresAt.
// Claims validation is disabled so the expiry claim can be inspected even for a
// deliberately expired token; only the signature is verified.
func tokenExpiry(t *testing.T, cfg *config.Config, tokenString string) time.Time {
	t.Helper()
	claims := &jwt.RegisteredClaims{}
	if _, err := jwt.ParseWithClaims(tokenString, claims, func(*jwt.Token) (interface{}, error) {
		return []byte(cfg.JWTSecretKey), nil
	}, jwt.WithoutClaimsValidation()); err != nil {
		t.Fatalf("tokenExpiry: parsing token failed: %v", err)
	}
	if claims.ExpiresAt == nil {
		t.Fatalf("tokenExpiry: token has no ExpiresAt claim")
	}
	return claims.ExpiresAt.Time
}

func TestCreateAccessTokenRoundTrip(t *testing.T) {
	cfg := newJWTTestConfig("round-trip-secret", "HS256")
	const subject = "user-abc-123"

	token, err := CreateAccessToken(cfg, subject, 15)
	if err != nil {
		t.Fatalf("CreateAccessToken error: %v", err)
	}
	if token == "" {
		t.Fatalf("CreateAccessToken returned an empty token")
	}

	got, err := ParseSubject(cfg, token)
	if err != nil {
		t.Fatalf("ParseSubject error: %v", err)
	}
	if got != subject {
		t.Fatalf("ParseSubject = %q, want %q", got, subject)
	}
}

func TestParseSubjectRejectsWrongSecret(t *testing.T) {
	signCfg := newJWTTestConfig("the-signing-secret", "HS256")
	verifyCfg := newJWTTestConfig("a-different-secret", "HS256")

	token, err := CreateAccessToken(signCfg, "user-xyz", 15)
	if err != nil {
		t.Fatalf("CreateAccessToken error: %v", err)
	}

	got, err := ParseSubject(verifyCfg, token)
	if err == nil {
		t.Fatalf("ParseSubject with a different secret should fail, got subject %q and nil error", got)
	}
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("ParseSubject error = %v, want ErrInvalidToken (%v)", err, ErrInvalidToken)
	}
	if got != "" {
		t.Fatalf("ParseSubject returned subject %q on failure, want empty string", got)
	}
}

func TestParseSubjectRejectsExpiredToken(t *testing.T) {
	// CreateAccessToken treats expiresMinutes <= 0 as "use the configured
	// default", so to mint an already-expired token both the argument and the
	// configured default must be negative.
	cfg := newJWTTestConfig("expiry-secret", "HS256")
	cfg.AccessTokenExpireMinutes = -10

	token, err := CreateAccessToken(cfg, "user-expired", -10)
	if err != nil {
		t.Fatalf("CreateAccessToken error: %v", err)
	}

	// Sanity check: the token really is stamped in the past.
	if exp := tokenExpiry(t, cfg, token); !exp.Before(time.Now()) {
		t.Fatalf("expected an expired token, but ExpiresAt = %v is not in the past", exp)
	}

	got, err := ParseSubject(cfg, token)
	if err == nil {
		t.Fatalf("ParseSubject on an expired token should fail, got subject %q and nil error", got)
	}
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("ParseSubject error = %v, want ErrInvalidToken (%v)", err, ErrInvalidToken)
	}
	if got != "" {
		t.Fatalf("ParseSubject returned subject %q on failure, want empty string", got)
	}
}

func TestParseSubjectRejectsInvalidTokenStrings(t *testing.T) {
	cfg := newJWTTestConfig("garbage-secret", "HS256")

	valid, err := CreateAccessToken(cfg, "user-valid", 15)
	if err != nil {
		t.Fatalf("CreateAccessToken error: %v", err)
	}

	// Tamper with the final character so the signature no longer verifies while
	// the token keeps its three-segment structure.
	repl := byte('A')
	if valid[len(valid)-1] == 'A' {
		repl = 'B'
	}
	tampered := valid[:len(valid)-1] + string(repl)

	cases := []struct {
		name  string
		token string
	}{
		{name: "empty string", token: ""},
		{name: "not a jwt", token: "this-is-not-a-jwt"},
		{name: "too few segments", token: "header.payload"},
		{name: "garbage segments", token: "aaa.bbb.ccc"},
		{name: "tampered signature", token: tampered},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseSubject(cfg, tc.token)
			if err == nil {
				t.Fatalf("ParseSubject(%q) should fail, got subject %q and nil error", tc.token, got)
			}
			if !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("ParseSubject(%q) error = %v, want ErrInvalidToken (%v)", tc.token, err, ErrInvalidToken)
			}
			if got != "" {
				t.Fatalf("ParseSubject(%q) returned subject %q on failure, want empty string", tc.token, got)
			}
		})
	}
}

func TestCreateAccessTokenCustomVsDefaultExpiry(t *testing.T) {
	cfg := newJWTTestConfig("expiry-compare-secret", "HS256") // default 1440 minutes
	const subject = "user-expiry"

	customToken, err := CreateAccessToken(cfg, subject, 5)
	if err != nil {
		t.Fatalf("CreateAccessToken (custom expiry) error: %v", err)
	}
	// expiresMinutes == 0 falls through to cfg.AccessTokenExpireMinutes.
	defaultToken, err := CreateAccessToken(cfg, subject, 0)
	if err != nil {
		t.Fatalf("CreateAccessToken (default expiry) error: %v", err)
	}

	// Both tokens must be valid and round-trip to the same subject.
	for _, tc := range []struct {
		name  string
		token string
	}{
		{name: "custom expiry", token: customToken},
		{name: "default expiry", token: defaultToken},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseSubject(cfg, tc.token)
			if err != nil {
				t.Fatalf("ParseSubject(%s) error: %v", tc.name, err)
			}
			if got != subject {
				t.Fatalf("ParseSubject(%s) = %q, want %q", tc.name, got, subject)
			}
		})
	}

	// The default (1440 min) lifetime must extend well beyond the 5-minute
	// custom one, proving expiresMinutes actually overrides the cfg default.
	customExp := tokenExpiry(t, cfg, customToken)
	defaultExp := tokenExpiry(t, cfg, defaultToken)
	if !defaultExp.After(customExp) {
		t.Fatalf("default-expiry token should outlive custom-expiry token: default ExpiresAt %v, custom ExpiresAt %v", defaultExp, customExp)
	}
}

func TestRoundTripAcrossAlgorithms(t *testing.T) {
	algorithms := []string{"HS256", "HS384", "HS512"}

	for _, alg := range algorithms {
		t.Run(alg, func(t *testing.T) {
			cfg := newJWTTestConfig("algorithm-secret", alg)
			subject := "user-" + alg

			token, err := CreateAccessToken(cfg, subject, 15)
			if err != nil {
				t.Fatalf("CreateAccessToken(%s) error: %v", alg, err)
			}

			got, err := ParseSubject(cfg, token)
			if err != nil {
				t.Fatalf("ParseSubject(%s) error: %v", alg, err)
			}
			if got != subject {
				t.Fatalf("ParseSubject(%s) = %q, want %q", alg, got, subject)
			}
		})
	}
}
