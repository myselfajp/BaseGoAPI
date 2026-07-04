package security

import "testing"

func TestGenerateNumericOTPLength(t *testing.T) {
	for _, length := range []int{1, 4, 6, 8} {
		otp, err := GenerateNumericOTP(length)
		if err != nil {
			t.Fatalf("GenerateNumericOTP(%d) error: %v", length, err)
		}
		if len(otp) != length {
			t.Fatalf("GenerateNumericOTP(%d) = %q, want length %d", length, otp, length)
		}
		for _, r := range otp {
			if r < '0' || r > '9' {
				t.Fatalf("OTP %q contains non-digit", otp)
			}
		}
	}
}

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("Password1")
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}
	if !VerifyPassword("Password1", hash) {
		t.Fatalf("VerifyPassword should succeed for the correct password")
	}
	if VerifyPassword("wrong", hash) {
		t.Fatalf("VerifyPassword should fail for the wrong password")
	}
	if VerifyPassword("Password1", "not-a-hash") {
		t.Fatalf("VerifyPassword should return false for a malformed hash")
	}
}

func TestGenerateURLSafeToken(t *testing.T) {
	a, err := GenerateURLSafeToken(32)
	if err != nil {
		t.Fatalf("GenerateURLSafeToken error: %v", err)
	}
	b, _ := GenerateURLSafeToken(32)
	if a == "" || a == b {
		t.Fatalf("tokens should be non-empty and unique: %q vs %q", a, b)
	}
}
