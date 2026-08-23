package baseconv

const (
	// AlphabetBase32 is Crockford's 32-character set. It does not contain I, L, O, or U.
	AlphabetBase32 = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

	// AlphabetBase62 defines the character set for base62 encoding.
	// It includes digits, uppercase letters, and lowercase letters for maximum character density.
	AlphabetBase62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

var (
	// Std32Encoding encodes with AlphabetBase32 and no padding.
	Std32Encoding = must(NewBaseEncoding(AlphabetBase32))
	// StdRaw32Encoding encodes with AlphabetBase32 and '=' padding. Unlike
	// encoding/base32.RawStdEncoding, Raw here means padded, not unpadded.
	StdRaw32Encoding = must(NewBaseEncodingWithPadding(AlphabetBase32, '='))
)

var (
	// Std62Encoding encodes with AlphabetBase62 and no padding.
	Std62Encoding = must(NewBaseEncoding(AlphabetBase62))
	// StdRaw62Encoding is constructed with '=' padding, but the mathematical
	// encoder never emits padding. DecodeString still strips trailing '='.
	StdRaw62Encoding = must(NewBaseEncodingWithPadding(AlphabetBase62, '='))
)

func must(e *BaseEncoding, err error) *BaseEncoding {
	if err != nil {
		panic(err)
	}
	return e
}
