package baseconv

const (
	AlphabetBase32 = "0123456789ABCDEFGHJKLMNPQRSTVWXYZ"
	AlphabetBase62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

var (
	Std32Encoding    *BaseEncoding
	StdRaw32Encoding *BaseEncoding
)

var (
	Std62Encoding    *BaseEncoding
	StdRaw62Encoding *BaseEncoding
)

func init() {
	Std32Encoding, _ = NewBaseEncoding(AlphabetBase32)
	StdRaw32Encoding, _ = NewBaseEncodingWithPadding(AlphabetBase32, '=')

	Std62Encoding, _ = NewBaseEncoding(AlphabetBase62)
	StdRaw62Encoding, _ = NewBaseEncodingWithPadding(AlphabetBase62, '=')
}
