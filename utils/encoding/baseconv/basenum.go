package baseconv

const (
	// AlphabetBase32 defines the character set for base32 encoding using Crockford's variant.
	// It excludes ambiguous characters (I, L, O, U) to prevent confusion in human-readable contexts.
	AlphabetBase32 = "0123456789ABCDEFGHJKLMNPQRSTVWXYZ"

	// AlphabetBase62 defines the character set for base62 encoding.
	// It includes digits, uppercase letters, and lowercase letters for maximum character density.
	AlphabetBase62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

var (
	// Std32Encoding is the standard base32 encoder using Crockford's alphabet without padding.
	Std32Encoding = must(NewBaseEncoding(AlphabetBase32))
	// StdRaw32Encoding is the standard base32 encoder with padding using '=' character.
	StdRaw32Encoding = must(NewBaseEncodingWithPadding(AlphabetBase32, '='))
)

var (
	// Std62Encoding is the standard base62 encoder without padding.
	Std62Encoding = must(NewBaseEncoding(AlphabetBase62))
	// StdRaw62Encoding is the standard base62 encoder with padding using '=' character.
	StdRaw62Encoding = must(NewBaseEncodingWithPadding(AlphabetBase62, '='))
)

func must(e *BaseEncoding, err error) *BaseEncoding {
	if err != nil {
		panic(err)
	}
	return e
}
