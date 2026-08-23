// Package secure is bcrypt password hashing, a display mask, and
// crypto/rand alphanumeric strings.
//
// CryptPassword / IsPasswordMatch use bcrypt default cost. Hashing never
// returns the plaintext on error. bcrypt's 72-byte input limit applies.
// CensorString keeps the first and last rune and fills the middle with '*'
// to outLength; outLength < 2 or empty src yields all stars. RandString
// panics on entropy failure and on negative length.
package secure

import (
	"golang.org/x/crypto/bcrypt"
)

// CryptPassword hashes a plain text password using bcrypt with default cost.
// It returns the hashed password string, or an error if hashing fails (for example
// when the password exceeds bcrypt's 72-byte limit). It never falls back to returning
// the plain text password.
// The bcrypt algorithm includes salt generation and is resistant to rainbow table attacks.
func CryptPassword(pwd string) (string, error) {
	cyPwd, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(cyPwd), nil
}

// IsPasswordMatch reports whether pwd matches the bcrypt hash cyPwd.
// Comparison is constant-time (bcrypt).
func IsPasswordMatch(pwd, cyPwd string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(cyPwd), []byte(pwd))
	return err == nil
}
