package service

import "testing"

func TestMaskEmail(t *testing.T) {
	cases := map[string]string{
		"john.doe@example.com": "j***e@example.com",
		"ab@example.com":       "a***@example.com",
		"a@example.com":        "a***@example.com",
		"not-an-email":         "not-an-email",
	}
	for input, want := range cases {
		if got := maskEmail(input); got != want {
			t.Fatalf("maskEmail(%q) = %q, want %q", input, got, want)
		}
	}
}
