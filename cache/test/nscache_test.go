package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-sphere/sphere/cache/nscache"
)

// TestNSCacheAdversarialIsolation verifies that NSCache maintains strict namespace
// isolation under concurrent operations across sibling namespaces on the same backend.
func TestNSCacheAdversarialIsolation(t *testing.T) {
	t.Parallel()

	for _, factory := range statefulByteCacheFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			backend := factory.new(t)

			nsA := nscache.NewNSCache("ns_a", backend)
			nsB := nscache.NewNSCache("ns_b", backend)

			const n = 50
			for i := range n {
				key := fmt.Sprintf("k_%d", i)
				if err := nsA.Set(ctx, key, []byte("val_a")); err != nil {
					t.Fatalf("nsA.Set: %v", err)
				}
				if err := nsB.Set(ctx, key, []byte("val_b")); err != nil {
					t.Fatalf("nsB.Set: %v", err)
				}
			}

			for i := range n {
				key := fmt.Sprintf("k_%d", i)
				valA, foundA, errA := nsA.Get(ctx, key)
				if errA != nil || !foundA || string(valA) != "val_a" {
					t.Fatalf("nsA mismatch for %s: found=%v, val=%s, err=%v", key, foundA, string(valA), errA)
				}
				valB, foundB, errB := nsB.Get(ctx, key)
				if errB != nil || !foundB || string(valB) != "val_b" {
					t.Fatalf("nsB mismatch for %s: found=%v, val=%s, err=%v", key, foundB, string(valB), errB)
				}
			}
		})
	}
}
