package secure

import (
	"crypto/rand"
	"math/big"
)

// RandString generates a random string of the specified length using alphanumeric characters.
// It uses crypto/rand as the entropy source with rejection sampling to avoid modulo bias,
// producing cryptographically secure, unpredictable strings suitable for tokens, passwords,
// or other security-sensitive applications.
// If length is less than or equal to 0, RandString returns an empty string.
// It panics if the system entropy source fails, since that is an unrecoverable condition.
func RandString(length int) string {
	if length <= 0 {
		return ""
	}
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	res := make([]byte, length)
	max := big.NewInt(int64(len(chars)))
	for i := range res {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic("secure: crypto/rand failed: " + err.Error())
		}
		res[i] = chars[n.Int64()]
	}
	return string(res)
}
