package secure

import (
	"strings"
	"unicode/utf8"
)

// CensorString masks a string by replacing middle characters with asterisks while preserving
// the first and last characters for recognition. The output length is controlled by outLength parameter.
// If the source string has only one character, it's repeated at both ends with asterisks in between.
// The result has outLength runes regardless of len(src). outLength < 2 yields
// max(outLength, 0) asterisks, and an empty src yields outLength asterisks.
// Characters are counted as runes, so multi-byte text is not split.
func CensorString(src string, outLength int) string {
	if outLength < 2 {
		return strings.Repeat("*", max(outLength, 0))
	}
	first, size := utf8.DecodeRuneInString(src)
	if size == 0 {
		return strings.Repeat("*", outLength)
	}
	last, _ := utf8.DecodeLastRuneInString(src)
	var b strings.Builder
	b.Grow(utf8.RuneLen(first) + (outLength - 2) + utf8.RuneLen(last))
	b.WriteRune(first)
	for range outLength - 2 {
		b.WriteByte('*')
	}
	b.WriteRune(last)
	return b.String()
}
