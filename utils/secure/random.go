package secure

import "math/rand/v2"

// RandString generates a random string of the specified length using alphanumeric characters.
// It uses cryptographically secure random number generation for creating unpredictable strings
// suitable for tokens, passwords, or other security-sensitive applications.
func RandString(length int) string {
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	res := make([]byte, length)
	for i := range res {
		res[i] = chars[rand.IntN(len(chars))]
	}
	return string(res)
}
