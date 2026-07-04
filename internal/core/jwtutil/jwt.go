// Package jwtutil creates and validates JWT access tokens. It is the Go
// equivalent of app/core/jwt.py (plus the token-decoding logic that lived in
// app/service/deps.py).
package jwtutil

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/myselfajp/BaseGoAPI/internal/config"
)

// ErrInvalidToken is returned when a token cannot be validated.
var ErrInvalidToken = errors.New("could not validate credentials")

// CreateAccessToken issues a signed JWT whose subject is the given value.
// When expiresMinutes is <= 0 the configured default lifetime is used.
func CreateAccessToken(cfg *config.Config, subject string, expiresMinutes int) (string, error) {
	minutes := expiresMinutes
	if minutes <= 0 {
		minutes = cfg.AccessTokenExpireMinutes
	}

	claims := jwt.RegisteredClaims{
		Subject:   subject,
		ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(time.Duration(minutes) * time.Minute)),
	}

	method := signingMethod(cfg.JWTAlgorithm)
	token := jwt.NewWithClaims(method, claims)
	return token.SignedString([]byte(cfg.JWTSecretKey))
}

// ParseSubject validates the token and returns its subject claim.
// WithStrictDecoding rejects non-canonical base64 (trailing padding bits), so a
// token altered in those bits is refused instead of decoding to the same bytes.
func ParseSubject(cfg *config.Config, tokenString string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(cfg.JWTSecretKey), nil
	},
		jwt.WithValidMethods([]string{"HS256", "HS384", "HS512"}),
		jwt.WithStrictDecoding(),
	)
	if err != nil || !token.Valid || claims.Subject == "" {
		return "", ErrInvalidToken
	}
	return claims.Subject, nil
}

// signingMethod maps the configured algorithm string to a JWT signing method,
// defaulting to HS256.
func signingMethod(alg string) jwt.SigningMethod {
	switch alg {
	case "HS384":
		return jwt.SigningMethodHS384
	case "HS512":
		return jwt.SigningMethodHS512
	default:
		return jwt.SigningMethodHS256
	}
}
