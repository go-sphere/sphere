package baseconv

const (
	AlphabetBase32 = "0123456789ABCDEFGHJKLMNPQRSTVWXYZ"
	AlphabetBase62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

var (
	Std32Encoding    = must(NewBaseEncoding(AlphabetBase32))
	StdRaw32Encoding = must(NewBaseEncodingWithPadding(AlphabetBase32, '='))
)

var (
	Std62Encoding    = must(NewBaseEncoding(AlphabetBase62))
	StdRaw62Encoding = must(NewBaseEncodingWithPadding(AlphabetBase62, '='))
)

func must(e *BaseEncoding, err error) *BaseEncoding {
	if err != nil {
		panic(err)
	}
	return e
}
