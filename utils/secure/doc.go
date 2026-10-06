// Package secure provides bcrypt password hashing, a display mask for
// sensitive strings, and crypto/rand alphanumeric strings.
//
// # Usage
//
//	import "github.com/go-sphere/sphere/utils/secure"
//
//	hash, err := secure.CryptPassword(plain) // store hash, never plain
//	if err != nil {
//		return err // e.g. plain longer than 72 bytes
//	}
//	ok := secure.IsPasswordMatch(attempt, hash)
//
//	token := secure.RandString(32)                   // unpredictable [a-zA-Z0-9]{32}
//	masked := secure.CensorString("13812345678", 11) // "1*********8"
//
// # Rules
//
//   - CryptPassword and IsPasswordMatch use bcrypt at its default cost.
//     bcrypt's 72-byte input limit is enforced: CryptPassword returns an
//     error and IsPasswordMatch returns false for longer input, rather than
//     silently truncating.
//   - CensorString keeps the first and last rune and fills the middle with
//     '*' up to outLength runes; outLength < 2 or an empty src yields only
//     stars.
//   - RandString panics if the system entropy source fails; a non-positive
//     length yields "".
//
// All functions are stateless and safe for concurrent use.
package secure
