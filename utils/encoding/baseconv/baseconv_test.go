package baseconv

import (
	"encoding/base64"
	"testing"
)

func TestBaseEncoding_Encode(t *testing.T) {
	encoding, err := NewBaseEncodingWithPadding("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/", '=')
	if err != nil {
		t.Fatalf("Failed to create base encoding: %v", err)
	}
	demo := encoding.EncodeToString([]byte("Hello, World!"))
	demo2 := base64.StdEncoding.EncodeToString([]byte("Hello, World!"))
	if demo != demo2 {
		t.Errorf("Expected %s, got %s", demo2, demo)
	} else {
		t.Logf("Base64 encoding of 'Hello, World!' is %s", demo)
	}
}
