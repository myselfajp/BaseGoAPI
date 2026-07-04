package security

import (
	"crypto/rand"
	"encoding/base64"
	"math/big"
)

// GenerateURLSafeToken returns a cryptographically-secure, URL-safe random
// token. It is the Go equivalent of Python's secrets.token_urlsafe(nBytes).
func GenerateURLSafeToken(nBytes int) (string, error) {
	if nBytes <= 0 {
		nBytes = 32
	}
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// GenerateNumericOTP returns a random numeric OTP of the requested length,
// mirroring the behaviour of the FastAPI AuthService._generate_otp_code helper.
func GenerateNumericOTP(length int) (string, error) {
	if length <= 1 {
		// Single digit in the range 1..9 (never a leading zero).
		n, err := rand.Int(rand.Reader, big.NewInt(9))
		if err != nil {
			return "", err
		}
		return big.NewInt(n.Int64() + 1).String(), nil
	}

	rangeStart := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(length-1)), nil)
	rangeEnd := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(length)), nil)
	span := new(big.Int).Sub(rangeEnd, rangeStart) // 10^n - 10^(n-1)

	offset, err := rand.Int(rand.Reader, span)
	if err != nil {
		return "", err
	}
	return new(big.Int).Add(rangeStart, offset).String(), nil
}
