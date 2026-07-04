// Package security provides password hashing and secure token/OTP generation.
// It is the Go equivalent of app/core/security.py.
package security

import "golang.org/x/crypto/bcrypt"

// HashPassword hashes a plain-text password using bcrypt.
func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// VerifyPassword reports whether the plain-text password matches the hash.
// It returns false for any mismatch or malformed hash instead of an error,
// mirroring the Python implementation.
func VerifyPassword(plain, hashed string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}
