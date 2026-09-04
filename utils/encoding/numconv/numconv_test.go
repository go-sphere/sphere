package numconv

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/go-sphere/sphere/utils/encoding/baseconv"
)

func Test_bytesToInt64(t *testing.T) {
	t.Parallel()

	if _, err := bytesToInt64(int64ToBytes(42)); err != nil {
		t.Fatalf("the canonical eight-byte form must decode: %v", err)
	}

	// Anything other than the eight bytes int64ToBytes produces is not an
	// encoding this package emitted, and accepting it would make decoding
	// many-to-one.
	for _, size := range []int{0, 1, 4, 7, 9} {
		if _, err := bytesToInt64(make([]byte, size)); !errors.Is(err, ErrNonCanonical) {
			t.Errorf("bytesToInt64(%d bytes) error = %v, want ErrNonCanonical", size, err)
		}
	}
}

// TestDecodeRejectsNonCanonicalForms pins that an encoded identifier has exactly
// one spelling.
//
// These strings all used to decode successfully to a value another string
// already denoted, so two distinct identifiers referred to the same entity —
// and any deduplication, idempotency key or cache lookup done on the string
// silently treated them as different.
func TestDecodeRejectsNonCanonicalForms(t *testing.T) {
	t.Parallel()

	t.Run("base32", func(t *testing.T) {
		canonical := Int64ToBase32(1)
		got, err := Base32ToInt64(canonical)
		if err != nil || got != 1 {
			t.Fatalf("canonical form must round-trip: got=%d err=%v", got, err)
		}

		// Differs from the canonical form only in bits the decoder discarded.
		alias := canonical[:len(canonical)-1] + "3"
		if alias == canonical {
			t.Fatal("test setup: alias matches the canonical form")
		}
		if _, err := Base32ToInt64(alias); !errors.Is(err, baseconv.ErrNonCanonical) {
			t.Errorf("Base32ToInt64(%q) error = %v, want ErrNonCanonical", alias, err)
		}

		// One character longer than any value needs.
		overlong := canonical + "Z"
		if _, err := Base32ToInt64(overlong); err == nil {
			t.Errorf("Base32ToInt64(%q) must not decode", overlong)
		}
	})

	t.Run("base62", func(t *testing.T) {
		canonical := Int64ToBase62(5)
		got, err := Base62ToInt64(canonical)
		if err != nil || got != 5 {
			t.Fatalf("canonical form must round-trip: got=%d err=%v", got, err)
		}

		// The same value written without its leading zeros.
		if _, err := Base62ToInt64("5"); !errors.Is(err, ErrNonCanonical) {
			t.Errorf("Base62ToInt64(%q) error = %v, want ErrNonCanonical", "5", err)
		}
	})
}

// TestRoundTripAcrossInt64Domain pins that tightening the decoder did not
// narrow what legitimately encodes.
func TestRoundTripAcrossInt64Domain(t *testing.T) {
	t.Parallel()

	values := []int64{0, 1, -1, 42, -42, math.MaxInt64, math.MinInt64, math.MaxInt32, math.MinInt32}
	for _, v := range values {
		got32, err := Base32ToInt64(Int64ToBase32(v))
		if err != nil || got32 != v {
			t.Errorf("base32 round-trip of %d: got=%d err=%v", v, got32, err)
		}
		got62, err := Base62ToInt64(Int64ToBase62(v))
		if err != nil || got62 != v {
			t.Errorf("base62 round-trip of %d: got=%d err=%v", v, got62, err)
		}
	}
}

// TestRandomGenerators pin the documented contract of the two random-string
// helpers: exact length, characters drawn only from the advertised alphabet,
// and empty output for non-positive lengths.
func TestRandomGenerators(t *testing.T) {
	tests := []struct {
		name     string
		generate func(int) string
		alphabet string
	}{
		{name: "base32", generate: RandomBase32, alphabet: baseconv.AlphabetBase32},
		{name: "base62", generate: RandomBase62, alphabet: baseconv.AlphabetBase62},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("length and alphabet", func(t *testing.T) {
				for _, length := range []int{1, 16, 64} {
					got := tt.generate(length)
					if len(got) != length {
						t.Fatalf("generate(%d) has length %d", length, len(got))
					}
					for _, r := range got {
						if !strings.ContainsRune(tt.alphabet, r) {
							t.Fatalf("generate(%d) contains %q, not in the alphabet", length, r)
						}
					}
				}
			})

			t.Run("non-positive length yields empty", func(t *testing.T) {
				for _, length := range []int{0, -1} {
					if got := tt.generate(length); got != "" {
						t.Fatalf("generate(%d) = %q, want empty", length, got)
					}
				}
			})
		})
	}
}
