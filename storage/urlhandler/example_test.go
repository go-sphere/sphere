package urlhandler_test

import (
	"errors"
	"fmt"

	"github.com/go-sphere/sphere/storage/urlhandler"
)

func ExampleNewHandler() {
	h, err := urlhandler.NewHandler("https://cdn.example.com/assets/")
	if err != nil {
		fmt.Println(err)
		return
	}
	u := h.GenerateURL("avatars/a.png")
	fmt.Println(u)
	fmt.Println(h.ExtractKeyFromURL(u))
	// Absolute URLs pass through GenerateURL unchanged.
	fmt.Println(h.GenerateURL("https://other.example.com/x.png"))
	// Output:
	// https://cdn.example.com/assets/avatars/a.png
	// avatars/a.png
	// https://other.example.com/x.png
}

func ExampleHandler_ExtractKeyFromURLWithMode() {
	h, err := urlhandler.NewHandler("https://cdn.example.com/assets")
	if err != nil {
		fmt.Println(err)
		return
	}
	foreign := "https://other.example.com/assets/a.png"

	_, err = h.ExtractKeyFromURLWithMode(foreign, true)
	fmt.Println("strict:", errors.Is(err, urlhandler.ErrHostVerificationFailed))

	key, err := h.ExtractKeyFromURLWithMode(foreign, false)
	fmt.Println("lenient:", key, err)

	// A bare key is returned as is, without its leading slash.
	key, err = h.ExtractKeyFromURLWithMode("/avatars/a.png", true)
	fmt.Println("bare key:", key, err)
	// Output:
	// strict: true
	// lenient: a.png <nil>
	// bare key: avatars/a.png <nil>
}
