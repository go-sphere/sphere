package baseconv

import (
	"bytes"
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

func TestMathematicalEncodingRoundTripLeadingZeros(t *testing.T) {
	t.Parallel()

	encoding, err := NewBaseEncoding(AlphabetBase62)
	if err != nil {
		t.Fatalf("NewBaseEncoding: %v", err)
	}

	tests := [][]byte{
		{0},
		{0, 0, 0},
		{0, 0, 1},
		{0, 0, 1, 2, 3},
	}
	for _, want := range tests {
		encoded := encoding.EncodeToString(want)
		got, err := encoding.DecodeString(encoded)
		if err != nil {
			t.Fatalf("DecodeString(%q): %v", encoded, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("round trip mismatch: input=%v encoded=%q got=%v", want, encoded, got)
		}
	}
}
