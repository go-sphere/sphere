// Package numconv turns int64 values (such as database IDs) into short,
// URL-safe strings and back.
//
// [Int64ToBase32] / [Base32ToInt64] use baseconv.Std32Encoding (Crockford
// alphabet, unpadded); [Int64ToBase62] / [Base62ToInt64] use
// baseconv.Std62Encoding. Values are encoded as their 8-byte big-endian two's
// complement form, so negative numbers round-trip too.
//
// # Usage
//
//	import "github.com/go-sphere/sphere/utils/encoding/numconv"
//
//	s := numconv.Int64ToBase62(id)
//	id2, err := numconv.Base62ToInt64(s)
//	if err != nil {
//		return err // not a string Int64ToBase62 produced
//	}
//
// # Rules
//
//   - Decoding accepts only canonical encodings: input that does not decode
//     to exactly 8 bytes fails with [ErrNonCanonical], so a short string like
//     "5" does not decode. Base32 leftover-bit errors match both
//     baseconv.ErrNonCanonical and ErrNonCanonical.
//   - Characters outside the alphabet fail with a plain (non-sentinel) error.
//   - [RandomBase32] and [RandomBase62] sample the alphabet with
//     math/rand/v2. They are not encodings of int64s and are not
//     cryptographically secure (use secure.RandString for secrets).
//
// All functions are stateless and safe for concurrent use.
package numconv
