// Package numconv provides utilities for converting 64-bit integers to/from base32 and base62 encodings.
// It includes functions for generating random strings in these encodings,
// making it useful for creating compact, URL-safe identifiers and tokens.
package numconv

import (
	"errors"
	"math/rand/v2"

	"github.com/go-sphere/sphere/utils/encoding/baseconv"
)

func int64ToBytes(n int64) []byte {
	bytes := make([]byte, 8)
	bytes[0] = byte(n >> 56)
	bytes[1] = byte(n >> 48)
	bytes[2] = byte(n >> 40)
	bytes[3] = byte(n >> 32)
	bytes[4] = byte(n >> 24)
	bytes[5] = byte(n >> 16)
	bytes[6] = byte(n >> 8)
	bytes[7] = byte(n)
	return bytes
}

func bytesToInt64(b []byte) (int64, error) {
	if len(b) > 8 {
		return 0, errors.New("byte slice too long, must be 8 bytes or less")
	}
	if len(b) < 8 {
		padded := make([]byte, 8)
		copy(padded[8-len(b):], b)
		b = padded
	}
	return int64(b[0])<<56 | int64(b[1])<<48 | int64(b[2])<<40 | int64(b[3])<<32 |
		int64(b[4])<<24 | int64(b[5])<<16 | int64(b[6])<<8 | int64(b[7]), nil
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
	if length <= 0 {
		return ""
	}
	result := make([]rune, length)
	for i := 0; i < length; i++ {
		result[i] = rune(baseconv.AlphabetBase32[rand.IntN(len(baseconv.AlphabetBase32))])
	}
	return string(result)
}

// RandomBase62 generates a random base62 string of the specified length.
// Returns an empty string if length is non-positive.
func RandomBase62(length int) string {
	if length <= 0 {
		return ""
	}
	result := make([]rune, length)
	for i := 0; i < length; i++ {
		result[i] = rune(baseconv.AlphabetBase62[rand.IntN(len(baseconv.AlphabetBase62))])
	}
	return string(result)
}
