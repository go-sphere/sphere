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
	// StdPadded32Encoding encodes with AlphabetBase32 and '=' padding.
	StdPadded32Encoding = must(NewBaseEncodingWithPadding(AlphabetBase32, '='))
)

var (
	// Std62Encoding encodes with AlphabetBase62 and no padding.
	Std62Encoding = must(NewBaseEncoding(AlphabetBase62))
)

func must(e *BaseEncoding, err error) *BaseEncoding {
	if err != nil {
		panic(err)
	}
	return e
}
