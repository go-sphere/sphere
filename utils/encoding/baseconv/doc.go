// Package baseconv encodes and decodes byte slices with a caller-chosen
// alphabet, such as Crockford base32 or base62 for compact identifiers.
//
// Use the predefined encodings ([Std32Encoding], [Std62Encoding]) or build
// one with [NewBaseEncoding] / [NewBaseEncodingWithPadding]. A [BaseEncoding]
// is immutable after construction and safe for concurrent use.
//
// # Usage
//
//	import "github.com/go-sphere/sphere/utils/encoding/baseconv"
//
//	s := baseconv.Std62Encoding.EncodeToString([]byte("hi"))
//	data, err := baseconv.Std62Encoding.DecodeString(s)
//	if err != nil {
//		return err
//	}
//
// # Algorithms
//
// Power-of-two alphabets (2, 4, ..., 64 characters) use a bitwise path; with
// the same 32-character alphabet the output matches encoding/base32. Other
// lengths (base62) use a big-integer path that preserves leading zero bytes
// as leading alphabet[0] characters. Padding is emitted only on the bitwise
// path for 32- and 64-character alphabets.
//
// Decoding is strict: a character outside the alphabet is an error, and on
// the bitwise path input the encoder could not have produced (non-zero
// leftover bits, an extra character, wrong padding length) fails with
// [ErrNonCanonical]. The big-integer path has no equivalent canonical check
// beyond padding.
//
// [AlphabetBase32] is Crockford's set without I, L, O, or U. [StdRaw32Encoding]
// and [StdRaw62Encoding] use '=' padding. That is the opposite of
// encoding/base32.Raw*, which means unpadded.
package baseconv
