package baseconv

import (
	"bytes"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"strings"
	"sync"
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

func TestStressBaseconvConcurrency(t *testing.T) {
	const (
		numGoroutines = 50
		iterations    = 200
	)

	var wg sync.WaitGroup
	for g := range numGoroutines {
		wg.Go(func() {
			for i := range iterations {
				// Generate random payload
				length := (g*iterations + i) % 128
				data := make([]byte, length)
				_, _ = rand.Read(data)

				// Test Std32
				enc32 := Std32Encoding.EncodeToString(data)
				dec32, err := Std32Encoding.DecodeString(enc32)
				if err != nil {
					t.Errorf("Std32 decode error: %v", err)
				}
				if !bytes.Equal(data, dec32) {
					t.Errorf("Std32 mismatch on length %d", length)
				}

				// Test Std62
				enc62 := Std62Encoding.EncodeToString(data)
				dec62, err := Std62Encoding.DecodeString(enc62)
				if err != nil {
					t.Errorf("Std62 decode error: %v", err)
				}
				if !bytes.Equal(data, dec62) {
					t.Errorf("Std62 mismatch on length %d", length)
				}
			}
		})
	}
	wg.Wait()
}
