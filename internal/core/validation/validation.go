// Package validation provides field-level validators. It is the Go equivalent
// of app/core/validation.py. Each validator returns nil when the input is
// valid, or an error whose message is safe to return to the client.
package validation

import (
	"errors"
	"regexp"
	"strings"
)

var emailPattern = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

var (
	hasUpper = regexp.MustCompile(`[A-Z]`)
	hasLower = regexp.MustCompile(`[a-z]`)
	hasDigit = regexp.MustCompile(`\d`)
)

var phonePattern = regexp.MustCompile(`^\+?\d+$`)

// ValidateEmail checks the format of an email address.
func ValidateEmail(email string) error {
	if email == "" || len(email) < 3 {
		return errors.New("Email is too short")
	}
	if len(email) > 255 {
		return errors.New("Email is too long")
	}
	if !emailPattern.MatchString(email) {
		return errors.New("Invalid email format")
	}
	return nil
}

// ValidatePassword enforces the password strength policy:
//   - at least 8 characters (max 128)
//   - at least one uppercase letter
//   - at least one lowercase letter
//   - at least one digit
func ValidatePassword(password string) error {
	if password == "" {
		return errors.New("Password is required")
	}
	if len(password) < 8 {
		return errors.New("Password must be at least 8 characters long")
	}
	if len(password) > 128 {
		return errors.New("Password is too long (max 128 characters)")
	}
	if !hasUpper.MatchString(password) {
		return errors.New("Password must contain at least one uppercase letter")
	}
	if !hasLower.MatchString(password) {
		return errors.New("Password must contain at least one lowercase letter")
	}
	if !hasDigit.MatchString(password) {
		return errors.New("Password must contain at least one digit")
	}
	return nil
}

// ValidatePhoneNumber validates an optional international phone number.
// An empty phone number is considered valid (the field is optional).
func ValidatePhoneNumber(phone string) error {
	if phone == "" {
		return nil
	}

	replacer := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "", ".", "")
	cleaned := replacer.Replace(phone)

	if !phonePattern.MatchString(cleaned) {
		return errors.New("Phone number must contain only digits, spaces, dashes, or start with +")
	}

	digitsOnly := strings.TrimLeft(strings.TrimPrefix(cleaned, "+"), "0")
	if len(digitsOnly) < 7 {
		return errors.New("Phone number is too short (minimum 7 digits)")
	}
	if len(strings.TrimPrefix(cleaned, "+")) > 15 {
		return errors.New("Phone number is too long (maximum 15 digits)")
	}
	return nil
}

// SanitizeString trims whitespace and enforces a maximum length. It returns an
// empty string for input that is empty after trimming.
func SanitizeString(text string, maxLength int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if maxLength > 0 && len(text) > maxLength {
		text = text[:maxLength]
	}
	return text
}
