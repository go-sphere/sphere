package numconv

import (
	"github.com/TBXark/sphere/utils/encoding/baseconv"
	"math/rand/v2"
	"testing"
)

func TestInt64ToBase62(t *testing.T) {
	for i := 0; i < 10; i++ {
		num := rand.Int64()
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
	for i := 0; i < 10; i++ {
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
	encoding, err := baseconv.NewBaseEncoding("abcdefghijklmnopqrstuvwxyz")
	if err != nil {
		t.Fatalf("failed to create base encoding: %v", err)
	}
	words := []string{
		"binding",
		"errors",
		"options",
	}
	for _, word := range words {
		raw, rErr := encoding.DecodeString(word)
		if rErr != nil {
			t.Fatalf("failed to decode string: %v", rErr)
		}
		num, rErr := bytesToInt64(raw)
		if rErr != nil {
			t.Fatalf("failed to convert bytes to int64: %v", rErr)
		}
		for num > 2147483646 {
			num /= 3
		}
		num /= 300
		t.Logf("%s: %d", word, num)
	}
}
