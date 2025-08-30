// Package secure provides security-related utilities including password hashing,
// string censoring, and random string generation for tokens and passwords.
// It uses industry-standard algorithms like bcrypt for secure password storage.
package secure

import (
	"golang.org/x/crypto/bcrypt"
)

// CryptPassword hashes a plain text password using bcrypt with default cost.
// It returns the hashed password string, or the original password if hashing fails.
// The bcrypt algorithm includes salt generation and is resistant to rainbow table attacks.
func CryptPassword(pwd string) string {
	cyPwd, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	if err != nil {
		return pwd
	}
	return string(cyPwd)
}

// IsPasswordMatch verifies if a plain text password matches the provided bcrypt hash.
// It returns true if the password matches the hash, false otherwise.
// This function is safe against timing attacks due to bcrypt's constant-time comparison.
func IsPasswordMatch(pwd, cyPwd string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(cyPwd), []byte(pwd))
	return err == nil
}
