// Package numconv encodes int64 values as 8-byte big-endian then base32 or
// base62 (unpadded Std32 / Std62). Decode requires exactly 8 bytes or
// returns ErrNonCanonical. A short string like "5" does not decode.
// Base32 leftover bits fail with baseconv.ErrNonCanonical.
//
// RandomBase32 and RandomBase62 sample the alphabet with math/rand/v2.
// They are not encodings of int64s and are not cryptographically secure
// (use secure.RandString for secrets).
package numconv

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand/v2"

	"github.com/go-sphere/sphere/utils/encoding/baseconv"
)

func int64ToBytes(n int64) []byte {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, uint64(n))
	return bytes
}

// ErrNonCanonical reports an encoded string that is not the one Int64ToBase32 or
// Int64ToBase62 would produce for the value it decodes to.
var ErrNonCanonical = errors.New("numconv: non-canonical encoding")

func bytesToInt64(b []byte) (int64, error) {
	// Exactly eight bytes, no padding. Int64ToBase32/62 always encode the full
	// eight-byte representation, so anything shorter is not an encoding this
	// package produced. Left-padding it instead made decoding many-to-one:
	// "5" and "00000005" both yielded 5, so two distinct strings denoted the
	// same value and slipped past any deduplication, idempotency check or cache
	// lookup performed on the string — which is precisely what these compact
	// identifiers are for.
	if len(b) != 8 {
		return 0, fmt.Errorf("%w: decoded %d bytes, want 8", ErrNonCanonical, len(b))
	}
	return int64(binary.BigEndian.Uint64(b)), nil
}

// Int64ToBase32 converts a 64-bit integer to its base32 string representation.
// It uses the standard base32 encoding with Crockford's alphabet.
func Int64ToBase32(n int64) string {
	return baseconv.Std32Encoding.EncodeToString(int64ToBytes(n))
}

// Int64ToBase62 converts a 64-bit integer to its base62 string representation.
// It uses the standard base62 encoding with alphanumeric characters.
func Int64ToBase62(n int64) string {
	return baseconv.Std62Encoding.EncodeToString(int64ToBytes(n))
}

// Base32ToInt64 converts a base32 encoded string back to a 64-bit integer.
// Returns an error if the string contains invalid base32 characters or cannot be decoded.
func Base32ToInt64(s string) (int64, error) {
	bytes, err := baseconv.Std32Encoding.DecodeString(s)
	if err != nil {
		return 0, err
	}
	return bytesToInt64(bytes)
}

// Base62ToInt64 converts a base62 encoded string back to a 64-bit integer.
// Returns an error if the string contains invalid base62 characters or cannot be decoded.
func Base62ToInt64(s string) (int64, error) {
	bytes, err := baseconv.Std62Encoding.DecodeString(s)
	if err != nil {
		return 0, err
	}
	return bytesToInt64(bytes)
}

// RandomBase32 generates a random base32 string of the specified length.
// Returns an empty string if length is non-positive.
func RandomBase32(length int) string {
	return randomBase(baseconv.AlphabetBase32, length)
}

// RandomBase62 generates a random base62 string of the specified length.
// Returns an empty string if length is non-positive.
func RandomBase62(length int) string {
	return randomBase(baseconv.AlphabetBase62, length)
}

func randomBase(alphabet string, length int) string {
	if length <= 0 {
		return ""
	}
	result := make([]byte, length)
	for i := range length {
		result[i] = alphabet[rand.IntN(len(alphabet))]
	}
	return string(result)
}
