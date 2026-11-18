package baseconv

import (
	"bytes"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"math/big"
	"math/rand/v2"
	"slices"
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
		if got, want := StdPadded32Encoding.EncodeToString(input), ref.EncodeToString(input); got != want {
			t.Errorf("StdPadded32Encoding.EncodeToString(%q) = %q, want %q", input, got, want)
		}
		if got, want := Std32Encoding.EncodeToString(input), ref.WithPadding(base32.NoPadding).EncodeToString(input); got != want {
			t.Errorf("Std32Encoding.EncodeToString(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestStd32EncodingRoundTrip(t *testing.T) {
	t.Parallel()

	for _, encoding := range map[string]*BaseEncoding{
		"Std32Encoding":       Std32Encoding,
		"StdPadded32Encoding": StdPadded32Encoding,
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

	canonical32 := StdPadded32Encoding.EncodeToString([]byte{0x00}) // two chars + six pads
	if _, err := StdPadded32Encoding.DecodeString(canonical32); err != nil {
		t.Fatalf("DecodeString(%q) = %v, want nil", canonical32, err)
	}
	for _, input := range []string{
		strings.TrimRight(canonical32, "="), // missing padding
		canonical32 + "=",                   // extra padding
		strings.Repeat("=", 8),              // padding only, collides with ""
	} {
		if _, err := StdPadded32Encoding.DecodeString(input); !errors.Is(err, ErrNonCanonical) {
			t.Errorf("DecodeString(%q) err = %v, want ErrNonCanonical", input, err)
		}
	}

	// The mathematical path never emits padding, so any trailing '=' is
	// non-canonical even when the encoding was constructed with a pad char.
	padded62, err := NewBaseEncodingWithPadding(AlphabetBase62, '=')
	if err != nil {
		t.Fatalf("NewBaseEncodingWithPadding(base62): %v", err)
	}
	canonical62 := padded62.EncodeToString([]byte{0x01})
	if _, err := padded62.DecodeString(canonical62); err != nil {
		t.Fatalf("DecodeString(%q) = %v, want nil", canonical62, err)
	}
	if _, err := padded62.DecodeString(canonical62 + "="); !errors.Is(err, ErrNonCanonical) {
		t.Errorf("DecodeString(%q+\"=\") err = %v, want ErrNonCanonical", canonical62, err)
	}
}

func TestDecodeStringInvalidCharPrecedesPaddingLength(t *testing.T) {
	t.Parallel()

	_, err := StdPadded32Encoding.DecodeString("!==")
	if err == nil {
		t.Fatal("DecodeString(\"!==\") succeeded, want invalid character error")
	}
	if errors.Is(err, ErrNonCanonical) {
		t.Fatalf("DecodeString(\"!==\") = %v, want invalid character error before padding length check", err)
	}
	if !strings.Contains(err.Error(), "invalid character '!' at position 0") {
		t.Fatalf("DecodeString(\"!==\") = %v, want invalid character message", err)
	}
}

// TestMathematicalEncodingMatchesBigInt pins the non-power-of-two output
// against a math/big reference, so stored encodings stay valid: a leading zero
// byte becomes one alphabet[0] and the rest is the big-endian value in base.
func TestMathematicalEncodingMatchesBigInt(t *testing.T) {
	t.Parallel()

	reference := func(alphabet string, data []byte) string {
		zeros := 0
		for zeros < len(data) && data[zeros] == 0 {
			zeros++
		}
		n := new(big.Int).SetBytes(data)
		base := big.NewInt(int64(len(alphabet)))
		var digits []byte
		for mod := new(big.Int); n.Sign() > 0; {
			n.DivMod(n, base, mod)
			digits = append(digits, alphabet[mod.Int64()])
		}
		slices.Reverse(digits)
		return strings.Repeat(alphabet[:1], zeros) + string(digits)
	}

	rng := rand.New(rand.NewPCG(1, 2))
	for _, alphabet := range []string{"012", "0123456789", AlphabetBase62} {
		enc, err := NewBaseEncoding(alphabet)
		if err != nil {
			t.Fatalf("NewBaseEncoding(%q): %v", alphabet, err)
		}
		for range 2000 {
			data := make([]byte, rng.IntN(24))
			for i := range data {
				// Bias towards zero bytes to cover leading and inner zeros.
				if rng.IntN(4) != 0 {
					data[i] = byte(rng.IntN(256))
				}
			}
			want := reference(alphabet, data)
			got := enc.EncodeToString(data)
			if got != want {
				t.Fatalf("base %d EncodeToString(%x) = %q, want %q", len(alphabet), data, got, want)
			}
			back, err := enc.DecodeString(got)
			if err != nil || !bytes.Equal(back, data) {
				t.Fatalf("base %d DecodeString(%q) = %x, %v; want %x", len(alphabet), got, back, err, data)
			}
		}
	}
}

func TestBaseEncodingFull256ByteAlphabetAndHighByteRejection(t *testing.T) {
	t.Parallel()

	var fullAlphabet [256]byte
	for i := range fullAlphabet {
		fullAlphabet[i] = byte(i)
	}
	enc, err := NewBaseEncoding(string(fullAlphabet[:]))
	if err != nil {
		t.Fatalf("NewBaseEncoding(256 bytes): %v", err)
	}
	raw := []byte{0x00, 0x01, 0x80, 0xff}
	encoded := enc.EncodeToString(raw)
	decoded, err := enc.DecodeString(encoded)
	if err != nil || !bytes.Equal(decoded, raw) {
		t.Fatalf("round-trip(256) = (%x, %v), want %x", decoded, err, raw)
	}

	narrow, err := NewBaseEncoding("0123")
	if err != nil {
		t.Fatalf("NewBaseEncoding: %v", err)
	}
	if _, err := narrow.DecodeString("0\xff"); err == nil {
		t.Fatal("expected invalid character error for high byte 0xff")
	}
}

func TestBaseEncodingNilAndZeroValueGuards(t *testing.T) {
	t.Parallel()

	var nilEnc *BaseEncoding
	if got := nilEnc.EncodeToString([]byte("hi")); got != "" {
		t.Fatalf("nil EncodeToString = %q, want empty", got)
	}
	if _, err := nilEnc.DecodeString("hi"); err == nil {
		t.Fatal("nil DecodeString succeeded, want error")
	}

	var zero BaseEncoding
	if got := zero.EncodeToString([]byte("hi")); got != "" {
		t.Fatalf("zero EncodeToString = %q, want empty", got)
	}
	if _, err := zero.DecodeString("hi"); err == nil {
		t.Fatal("zero DecodeString succeeded, want error")
	}
	if _, bitwise := powerOfTwoBits(0); bitwise {
		t.Fatal("powerOfTwoBits(0) = true, want false")
	}
	if _, bitwise := powerOfTwoBits(1); bitwise {
		t.Fatal("powerOfTwoBits(1) = true, want false")
	}
}
