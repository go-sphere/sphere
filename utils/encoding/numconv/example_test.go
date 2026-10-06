package numconv_test

import (
	"errors"
	"fmt"

	"github.com/go-sphere/sphere/utils/encoding/numconv"
)

func ExampleInt64ToBase62() {
	s := numconv.Int64ToBase62(1234567890123)
	fmt.Println(s)

	id, err := numconv.Base62ToInt64(s)
	fmt.Println(id, err)
	// Output:
	// 00LjaL3EZ
	// 1234567890123 <nil>
}

func ExampleBase32ToInt64() {
	s := numconv.Int64ToBase32(-42)
	fmt.Println(s)

	id, err := numconv.Base32ToInt64(s)
	fmt.Println(id, err)

	// A short string is not an encoding this package produced.
	_, err = numconv.Base32ToInt64("5")
	fmt.Println(errors.Is(err, numconv.ErrNonCanonical))
	// Output:
	// ZZZZZZZZZZZXC
	// -42 <nil>
	// true
}
