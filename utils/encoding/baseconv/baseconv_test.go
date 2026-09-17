package baseconv

import (
	"bytes"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"strings"
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
	}
}

func TestAlphabetBase32IsCrockford(t *testing.T) {
	t.Parallel()

	if len(AlphabetBase32) != 32 {
		t.Fatalf("AlphabetBase32 must have 32 characters, got %d (%q)", len(AlphabetBase32), AlphabetBase32)
	}
	for _, ambiguous := range []string{"I", "L", "O", "U"} {
		if strings.Contains(AlphabetBase32, ambiguous) {
			t.Errorf("AlphabetBase32 must exclude ambiguous character %q", ambiguous)
		}
	}
}

// A 32-character alphabet must take the bitwise path, which makes the output
// interoperable with encoding/base32 using the same alphabet.
func TestStd32EncodingMatchesStdlib(t *testing.T) {
	t.Parallel()

	ref := base32.NewEncoding(AlphabetBase32)
	inputs := [][]byte{
		[]byte("f"),
		[]byte("fo"),
		[]byte("foo"),
		[]byte("foob"),
		[]byte("fooba"),
		[]byte("foobar"),
		{0x00, 0xff, 0x10, 0x80},
	}
	for _, input := range inputs {
		if got, want := StdRaw32Encoding.EncodeToString(input), ref.EncodeToString(input); got != want {
			t.Errorf("StdRaw32Encoding.EncodeToString(%q) = %q, want %q", input, got, want)
		}
		if got, want := Std32Encoding.EncodeToString(input), ref.WithPadding(base32.NoPadding).EncodeToString(input); got != want {
			t.Errorf("Std32Encoding.EncodeToString(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestStd32EncodingRoundTrip(t *testing.T) {
	t.Parallel()

	for _, encoding := range map[string]*BaseEncoding{
		"Std32Encoding":    Std32Encoding,
		"StdRaw32Encoding": StdRaw32Encoding,
	} {
		for _, want := range [][]byte{
			[]byte("f"),
			[]byte("foobar"),
			{0x00, 0x00, 0x01},
		} {
			encoded := encoding.EncodeToString(want)
			got, err := encoding.DecodeString(encoded)
			if err != nil {
				t.Fatalf("DecodeString(%q): %v", encoded, err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("round trip mismatch: input=%v encoded=%q got=%v", want, encoded, got)
			}
		}
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

func TestMathematicalEncodingRoundTripLongInput(t *testing.T) {
	t.Parallel()

	want := make([]byte, 1024)
	for i := 2; i < len(want); i++ {
		want[i] = byte(i*31 + 7)
	}

	encoded := Std62Encoding.EncodeToString(want)
	got, err := Std62Encoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("DecodeString(long input): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("long mathematical encoding did not round-trip")
	}
}

// TestDecodeStringRejectsNonCanonicalPadding pins that decoding stays
// one-to-one when '=' padding is configured: the pad count must be exactly
// what the encoder emits, so a value has a single spelling.
func TestDecodeStringRejectsNonCanonicalPadding(t *testing.T) {
	t.Parallel()

	canonical32 := StdRaw32Encoding.EncodeToString([]byte{0x00}) // two chars + six pads
	if _, err := StdRaw32Encoding.DecodeString(canonical32); err != nil {
		t.Fatalf("DecodeString(%q) = %v, want nil", canonical32, err)
	}
	for _, input := range []string{
		strings.TrimRight(canonical32, "="), // missing padding
		canonical32 + "=",                   // extra padding
		strings.Repeat("=", 8),              // padding only, collides with ""
	} {
		if _, err := StdRaw32Encoding.DecodeString(input); !errors.Is(err, ErrNonCanonical) {
			t.Errorf("DecodeString(%q) err = %v, want ErrNonCanonical", input, err)
		}
	}

	// The mathematical path never emits padding, so any trailing '=' is
	// non-canonical even though the encoding was constructed with a pad char.
	canonical62 := StdRaw62Encoding.EncodeToString([]byte{0x01})
	if _, err := StdRaw62Encoding.DecodeString(canonical62); err != nil {
		t.Fatalf("DecodeString(%q) = %v, want nil", canonical62, err)
	}
	if _, err := StdRaw62Encoding.DecodeString(canonical62 + "="); !errors.Is(err, ErrNonCanonical) {
		t.Errorf("DecodeString(%q+\"=\") err = %v, want ErrNonCanonical", canonical62, err)
	}
}
