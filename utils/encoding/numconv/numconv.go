package numconv

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"

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

// Encoded lengths of the eight-byte form. Base32 is fixed width. Base62 needs
// at most 11 digits for eight bytes, and each leading zero byte adds one "0"
// while removing at least one digit, so no encoding is longer. Checking these
// before decoding keeps an oversized input from reaching the quadratic base62
// decoder.
const (
	base32Len    = 13
	maxBase62Len = 11
)

// Int64ToBase32 converts a 64-bit integer to its base32 string representation.
// It uses the standard base32 encoding with Crockford's alphabet. The result
// always has 13 characters; Base32ToInt64 reverses it.
func Int64ToBase32(n int64) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(n))
	return baseconv.Std32Encoding.EncodeToString(buf[:])
}

// Int64ToBase62 converts a 64-bit integer to its base62 string representation.
// It uses the standard base62 encoding with alphanumeric characters. The
// length varies with the value, from 8 to 11 characters (small values keep
// leading "0" characters for their zero bytes); Base62ToInt64 reverses it.
func Int64ToBase62(n int64) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(n))
	return baseconv.Std62Encoding.EncodeToString(buf[:])
}

// Base32ToInt64 converts a base32 encoded string back to a 64-bit integer.
// Returns an error if the string contains invalid base32 characters or cannot be decoded.
// Non-canonical input, including any string Int64ToBase32 would not produce,
// fails with an error matching ErrNonCanonical.
func Base32ToInt64(s string) (int64, error) {
	if len(s) > base32Len {
		return 0, fmt.Errorf("%w: length %d, want %d", ErrNonCanonical, len(s), base32Len)
	}
	bytes, err := baseconv.Std32Encoding.DecodeString(s)
	if err != nil {
		if errors.Is(err, baseconv.ErrNonCanonical) {
			// Leftover bits describe exactly the non-canonical input this
			// package's ErrNonCanonical documents, so keep both sentinels
			// matchable rather than leaking only the baseconv one.
			return 0, fmt.Errorf("%w: %w", ErrNonCanonical, err)
		}
		return 0, err
	}
	return bytesToInt64(bytes)
}

// Base62ToInt64 converts a base62 encoded string back to a 64-bit integer.
// Returns an error if the string contains invalid base62 characters or cannot be decoded.
// Input longer than 11 characters, or that does not decode to exactly 8
// bytes, fails with an error matching ErrNonCanonical.
func Base62ToInt64(s string) (int64, error) {
	if len(s) > maxBase62Len {
		return 0, fmt.Errorf("%w: length %d, at most %d", ErrNonCanonical, len(s), maxBase62Len)
	}
	bytes, err := baseconv.Std62Encoding.DecodeString(s)
	if err != nil {
		return 0, err
	}
	return bytesToInt64(bytes)
}

// RandomBase32 generates a random base32 string of the specified length.
// Each character is drawn uniformly from crypto/rand using rejection sampling.
// Returns an empty string if length is non-positive, and panics if the system
// entropy source fails.
func RandomBase32(length int) string {
	return randomBase(baseconv.AlphabetBase32, length)
}

// RandomBase62 generates a random base62 string of the specified length.
// Each character is drawn uniformly from crypto/rand using rejection sampling.
// Returns an empty string if length is non-positive, and panics if the system
// entropy source fails.
func RandomBase62(length int) string {
	return randomBase(baseconv.AlphabetBase62, length)
}

func randomBase(alphabet string, length int) string {
	if length <= 0 || len(alphabet) == 0 {
		return ""
	}
	base := len(alphabet)
	maxByte := 256 - (256 % base)
	result := make([]byte, length)
	var stackBuf [64]byte
	buf := stackBuf[:]
	if length > len(stackBuf) {
		buf = make([]byte, min(length+length/8+8, 1024))
	}
	for i := 0; i < length; {
		need := min(len(buf), (length-i)+(length-i)/8+4)
		if _, err := rand.Read(buf[:need]); err != nil {
			panic("numconv: crypto/rand failed: " + err.Error())
		}
		for _, b := range buf[:need] {
			if int(b) >= maxByte {
				continue
			}
			result[i] = alphabet[int(b)%base]
			i++
			if i == length {
				break
			}
		}
	}
	return string(result)
}
