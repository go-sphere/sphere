package service

import "strings"

func Plural(word string) string {
	if word == "" {
		return word
	}
	// 特殊情况
	irregulars := map[string]string{
		"child":  "children",
		"person": "people",
		"man":    "men",
		"woman":  "women",
		"foot":   "feet",
		"tooth":  "teeth",
		"mouse":  "mice",
	}
	if plural, exists := irregulars[strings.ToLower(word)]; exists {
		return plural
	}
	// 基本规则
	lower := strings.ToLower(word)
	// 以 s, ss, sh, ch, x, z, o 结尾的加 es
	if strings.HasSuffix(lower, "s") || strings.HasSuffix(lower, "ss") ||
		strings.HasSuffix(lower, "sh") || strings.HasSuffix(lower, "ch") ||
		strings.HasSuffix(lower, "x") || strings.HasSuffix(lower, "z") ||
		strings.HasSuffix(lower, "o") {
		return word + "es"
	}
	// 以辅音字母 + y 结尾的，去掉y加ies
	if strings.HasSuffix(lower, "y") && len(word) > 1 {
		beforeY := lower[len(lower)-2]
		if beforeY != 'a' && beforeY != 'e' && beforeY != 'i' && beforeY != 'o' && beforeY != 'u' {
			return word[:len(word)-1] + "ies"
		}
	}
	// 以 f 或 fe 结尾的，去掉f/fe加ves
	if strings.HasSuffix(lower, "f") {
		return word[:len(word)-1] + "ves"
	}
	if strings.HasSuffix(lower, "fe") {
		return word[:len(word)-2] + "ves"
	}
	// 默认情况下加 s
	return word + "s"
}
