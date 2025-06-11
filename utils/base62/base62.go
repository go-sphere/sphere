package base62

import (
	"fmt"
)

const (
	base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

var (
	base62Map            map[rune]int64
	ErrInvalidBase62Char = fmt.Errorf("invalid character in base62 string")
)

func init() {
	base62Map = make(map[rune]int64)
	for i, char := range base62Chars {
		base62Map[char] = int64(i)
	}
}

func FromInt64(n int64) string {
	if n == 0 {
		return "0"
	}
	result := ""
	length := int64(len(base62Chars))
	for n > 0 {
		result = string(base62Chars[n%length]) + result
		n = n / length
	}

	return result
}

func ToInt64(s string) (int64, error) {
	var result int64 = 0
	length := int64(len(base62Chars))

	for _, char := range s {
		pos, exists := base62Map[char]
		if !exists {
			return 0, ErrInvalidBase62Char
		}
		result = result*length + pos
	}

	return result, nil
}
