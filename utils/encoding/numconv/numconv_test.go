package numconv

import (
	"errors"
	"math"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"

	"github.com/go-sphere/sphere/utils/encoding/baseconv"
)

func TestInt64ToBase62(t *testing.T) {
	numbers := []int64{0}
	for range 10 {
		numbers = append(numbers, rand.Int64())
	}
	for _, num := range numbers {
		encoded := Int64ToBase62(num)
		decoded, err := Base62ToInt64(encoded)
		if err != nil {
			t.Errorf("Error decoding %s: %v", encoded, err)
			continue
		}
		if num != decoded {
			t.Errorf("Expected %d, got %d for encoded %s", num, decoded, encoded)
		}
		t.Logf("numconv encoding of %d is %s", num, encoded)
	}
}

func TestInt64ToBase32(t *testing.T) {
	for range 10 {
		num := rand.Int64()
		encoded := Int64ToBase32(num)
		decoded, err := Base32ToInt64(encoded)
		if err != nil {
			t.Errorf("Error decoding %s: %v", encoded, err)
			continue
		}
		if num != decoded {
			t.Errorf("Expected %d, got %d for encoded %s", num, decoded, encoded)
		}
		t.Logf("numconv encoding of %d is %s", num, encoded)
	}
}

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
// empty output for non-positive lengths, and no repeats in practice — these
// functions exist to mint identifiers, where a duplicate is a broken token.
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

			t.Run("distinct draws", func(t *testing.T) {
				seen := make(map[string]struct{})
				for range 50 {
					got := tt.generate(24)
					if _, dup := seen[got]; dup {
						t.Fatalf("generator produced a duplicate: %q", got)
					}
					seen[got] = struct{}{}
				}
			})
		})
	}
}

// TestAdversarialNumconvRoundTripAndCorruptedInputs tests full 64-bit space,
// boundary numbers (MinInt64, MaxInt64, -1, 0, 1), and corrupted/non-canonical strings.
func TestAdversarialNumconvRoundTripAndCorruptedInputs(t *testing.T) {
	boundaries := []int64{
		math.MinInt64,
		math.MinInt64 + 1,
		-1000000,
		-256,
		-1,
		0,
		1,
		256,
		1000000,
		math.MaxInt64 - 1,
		math.MaxInt64,
	}

	for _, val := range boundaries {
		// Base32
		s32 := Int64ToBase32(val)
		dec32, err := Base32ToInt64(s32)
		if err != nil {
			t.Fatalf("Base32ToInt64 failed for %d: %v", val, err)
		}
		if dec32 != val {
			t.Fatalf("Base32 mismatch: want %d, got %d", val, dec32)
		}

		// Base62
		s62 := Int64ToBase62(val)
		dec62, err := Base62ToInt64(s62)
		if err != nil {
			t.Fatalf("Base62ToInt64 failed for %d: %v", val, err)
		}
		if dec62 != val {
			t.Fatalf("Base62 mismatch: want %d, got %d", val, dec62)
		}
	}

	// Adversarial non-canonical and corrupted strings
	corrupted := []string{
		"",
		"0",
		"1",
		"0000000",
		"invalid!!!",
		"========",
		"\x00\x00\x00\x00",
		"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", // too long
	}

	for _, s := range corrupted {
		if _, err := Base32ToInt64(s); err == nil {
			t.Errorf("Base32ToInt64(%q) should fail but returned nil error", s)
		}
		if _, err := Base62ToInt64(s); err == nil {
			t.Errorf("Base62ToInt64(%q) should fail but returned nil error", s)
		}
	}

	// Concurrent random draws and encoding
	const (
		numGoroutines = 50
		iterations    = 1000
	)
	var wg sync.WaitGroup
	for g := range numGoroutines {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			for range iterations {
				val := rand.Int64()
				s32 := Int64ToBase32(val)
				if got, err := Base32ToInt64(s32); err != nil || got != val {
					t.Errorf("Base32 roundtrip failed for %d (str=%q): got=%d, err=%v", val, s32, got, err)
				}
				s62 := Int64ToBase62(val)
				if got, err := Base62ToInt64(s62); err != nil || got != val {
					t.Errorf("Base62 roundtrip failed for %d (str=%q): got=%d, err=%v", val, s62, got, err)
				}

				// Random generator bounds
				r32 := RandomBase32(16)
				if len(r32) != 16 {
					t.Errorf("RandomBase32 length = %d, want 16", len(r32))
				}
				r62 := RandomBase62(16)
				if len(r62) != 16 {
					t.Errorf("RandomBase62 length = %d, want 16", len(r62))
				}

				// Non-positive lengths
				if RandomBase32(0) != "" || RandomBase32(-5) != "" {
					t.Errorf("RandomBase32 non-positive must return empty string")
				}
				if RandomBase62(0) != "" || RandomBase62(-5) != "" {
					t.Errorf("RandomBase62 non-positive must return empty string")
				}
			}
		}(g)
	}
	wg.Wait()
}
