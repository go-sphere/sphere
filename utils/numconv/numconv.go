package numconv

import (
	"fmt"
	"math/rand/v2"
)

const (
	base32Chars = "0123456789ABCDEFGHJKLMNPQRSTVWXYZ"
	base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

var (
	base32Map      map[rune]int64
	base62Map      map[rune]int64
	ErrInvalidChar = fmt.Errorf("invalid character in string")
)

func init() {
	base62Map = make(map[rune]int64)
	for i, char := range base62Chars {
		base62Map[char] = int64(i)
	}
	base32Map = make(map[rune]int64)
	for i, char := range base32Chars {
		base32Map[char] = int64(i)
	}
}

func fromInt64(n int64, chars string) string {
	if n == 0 {
		return "0"
	}
	result := ""
	length := int64(len(chars))
	for n > 0 {
		result = string(chars[n%length]) + result
		n = n / length
	}

	return result
}

func toInt64(s string, charMap map[rune]int64) (int64, error) {
	var result int64 = 0
	length := int64(len(charMap))

	for _, char := range s {
		pos, exists := charMap[char]
		if !exists {
			return 0, ErrInvalidChar
		}
		result = result*length + pos
	}

	return result, nil
}

func Int64ToBase32(n int64) string {
	return fromInt64(n, base32Chars)
}

func Int64ToBase62(n int64) string {
	return fromInt64(n, base62Chars)
}

func Base32ToInt64(s string) (int64, error) {
	return toInt64(s, base32Map)
}

func Base62ToInt64(s string) (int64, error) {
	return toInt64(s, base62Map)
}

func RandomBase32(length int) string {
	if length <= 0 {
		return ""
	}
	result := make([]rune, length)
	for i := 0; i < length; i++ {
		result[i] = rune(base32Chars[rand.IntN(len(base32Chars))])
	}
	return string(result)
}

func RandomBase62(length int) string {
	if length <= 0 {
		return ""
	}
	result := make([]rune, length)
	for i := 0; i < length; i++ {
		result[i] = rune(base62Chars[rand.IntN(len(base62Chars))])
	}
	return string(result)
}
