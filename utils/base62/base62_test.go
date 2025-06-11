package base62

import (
	"math/rand/v2"
	"testing"
)

func TestFromInt64(t *testing.T) {
	for i := 0; i < 10; i++ {
		num := rand.Int64()
		encoded := FromInt64(num)
		decoded, err := ToInt64(encoded)
		if err != nil {
			t.Errorf("Error decoding %s: %v", encoded, err)
			continue
		}
		if num != decoded {
			t.Errorf("Expected %d, got %d for encoded %s", num, decoded, encoded)
		}
		t.Logf("base62 encoding of %d is %s", num, encoded)
	}

}
