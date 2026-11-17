package secure

import (
	"crypto/rand"
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
	// Largest multiple of len(chars) (62) in [0, 256) so modulo mapping stays uniform.
	const maxByte = 256 - (256 % len(chars))
	res := make([]byte, length)
	var stackBuf [64]byte
	buf := stackBuf[:]
	if length > len(stackBuf) {
		buf = make([]byte, min(length+length/8+8, 1024))
	}
	for i := 0; i < length; {
		need := min(len(buf), (length-i)+(length-i)/8+4)
		if _, err := rand.Read(buf[:need]); err != nil {
			panic("secure: crypto/rand failed: " + err.Error())
		}
		for _, b := range buf[:need] {
			if int(b) >= maxByte {
				continue
			}
			res[i] = chars[int(b)%len(chars)]
			i++
			if i == length {
				break
			}
		}
	}
	return string(res)
}
