package validation

import "testing"

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid", "Password1", false},
		{"too short", "Pass1", true},
		{"no upper", "password1", true},
		{"no lower", "PASSWORD1", true},
		{"no digit", "Password", true},
		{"empty", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidatePassword(c.input)
			if (err != nil) != c.wantErr {
				t.Fatalf("ValidatePassword(%q) err=%v, wantErr=%v", c.input, err, c.wantErr)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	if err := ValidateEmail("user@example.com"); err != nil {
		t.Fatalf("expected valid email, got %v", err)
	}
	if err := ValidateEmail("not-an-email"); err == nil {
		t.Fatalf("expected error for invalid email")
	}
}

func TestValidatePhoneNumber(t *testing.T) {
	if err := ValidatePhoneNumber(""); err != nil {
		t.Fatalf("empty phone should be valid (optional), got %v", err)
	}
	if err := ValidatePhoneNumber("+1 (234) 567-890"); err != nil {
		t.Fatalf("expected valid phone, got %v", err)
	}
	if err := ValidatePhoneNumber("12345"); err == nil {
		t.Fatalf("expected error for too-short phone")
	}
	if err := ValidatePhoneNumber("abc"); err == nil {
		t.Fatalf("expected error for non-numeric phone")
	}
}

func TestSanitizeString(t *testing.T) {
	if got := SanitizeString("  hello  ", 255); got != "hello" {
		t.Fatalf("SanitizeString trim failed: %q", got)
	}
	if got := SanitizeString("   ", 255); got != "" {
		t.Fatalf("SanitizeString blank failed: %q", got)
	}
	if got := SanitizeString("abcdef", 3); got != "abc" {
		t.Fatalf("SanitizeString truncate failed: %q", got)
	}
}
