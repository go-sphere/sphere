package numconv

import (
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
