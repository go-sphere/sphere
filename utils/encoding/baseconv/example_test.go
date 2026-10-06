package baseconv_test

import (
	"errors"
	"fmt"

	"github.com/go-sphere/sphere/utils/encoding/baseconv"
)

func ExampleBaseEncoding_EncodeToString() {
	data := []byte("hi")
	fmt.Println(baseconv.Std32Encoding.EncodeToString(data))
	fmt.Println(baseconv.StdRaw32Encoding.EncodeToString(data)) // "Raw" means padded here
	fmt.Println(baseconv.Std62Encoding.EncodeToString(data))
	// Output:
	// D1MG
	// D1MG====
	// 6x7
}

func ExampleBaseEncoding_DecodeString() {
	decoded, err := baseconv.Std32Encoding.DecodeString("D1MG")
	fmt.Println(string(decoded), err)

	// "D1MH" differs from "D1MG" only in bits the encoder always leaves zero.
	_, err = baseconv.Std32Encoding.DecodeString("D1MH")
	fmt.Println(errors.Is(err, baseconv.ErrNonCanonical))

	_, err = baseconv.Std62Encoding.DecodeString("not-base62")
	fmt.Println(err)
	// Output:
	// hi <nil>
	// true
	// invalid character '-' at position 3
}

func ExampleNewBaseEncoding() {
	hex, err := baseconv.NewBaseEncoding("0123456789abcdef")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(hex.EncodeToString([]byte{0x0f, 0xa0}))

	_, err = baseconv.NewBaseEncoding("aa")
	fmt.Println(err)
	// Output:
	// 0fa0
	// alphabet contains duplicate characters
}
