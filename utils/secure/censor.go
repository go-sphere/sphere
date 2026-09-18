package secure

import "strings"

// CensorString masks a string by replacing middle characters with asterisks while preserving
// the first and last characters for recognition. The output length is controlled by outLength parameter.
// If the source string has only one character, it's repeated at both ends with asterisks in between.
func CensorString(src string, outLength int) string {
	if outLength < 2 {
		return strings.Repeat("*", max(outLength, 0))
	}
	runs := []rune(src)
	if len(runs) == 0 {
		return strings.Repeat("*", outLength)
	}
	last := runs[len(runs)-1]
	if len(runs) == 1 {
		last = runs[0]
	}
	return string(runs[0]) + strings.Repeat("*", outLength-2) + string(last)
}
