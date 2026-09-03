package metadata

import (
	"context"
	"math/rand/v2"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestMeta(t *testing.T) {
	testData := map[string]any{
		"key1": "value1",
		"key2": 123,
	}

	// Test case 1: WithMeta and MetaFrom
	ctx := WithMeta(context.Background(), testData)
	meta := MetaFrom(ctx)

	if !reflect.DeepEqual(meta, testData) {
		t.Errorf("MetaFrom() = %v, want %v", meta, testData)
	}

	// Test case 2: MetaFrom with no meta
	ctx = context.Background()
	meta = MetaFrom(ctx)
	if meta != nil {
		t.Errorf("MetaFrom() from context with no meta should be nil, but got %v", meta)
	}

	// Test case 3: MetaFrom with wrong type
	ctx = context.WithValue(context.Background(), metaContextKey, "not a map")
	meta = MetaFrom(ctx)
	if meta != nil {
		t.Errorf("MetaFrom() from context with wrong type should be nil, but got %v", meta)
	}
}

func TestWithMetaCopiesInput(t *testing.T) {
	original := map[string]any{
		"key1": "value1",
	}

	ctx := WithMeta(context.Background(), original)

	// Mutating the original map after WithMeta must not affect the stored copy.
	original["key1"] = "mutated"
	original["key2"] = "added"

	meta := MetaFrom(ctx)
	if meta["key1"] != "value1" {
		t.Errorf("stored meta key1 = %v, want %q (input mutation leaked into context)", meta["key1"], "value1")
	}
	if _, ok := meta["key2"]; ok {
		t.Errorf("stored meta unexpectedly gained key2 from input mutation")
	}
}

func TestMetaFromNilContext(t *testing.T) {
	var nilCtx context.Context
	meta := MetaFrom(nilCtx)
	if meta != nil {
		t.Errorf("MetaFrom(nil) = %v, want nil", meta)
	}
}

type customTestCtxKey struct{}
type unrelatedCtxKey struct{}

// TestAdversarialMetaFromNilAndConcurrentStress tests MetaFrom(nil) and context metadata
// under 150+ concurrent goroutines with -race enabled.
func TestAdversarialMetaFromNilAndConcurrentStress(t *testing.T) {
	// Baseline invariants
	var nilCtx context.Context
	if m := MetaFrom(nilCtx); m != nil {
		t.Fatalf("MetaFrom(nil) must return nil, got: %v", m)
	}

	const (
		numGoroutines = 150
		iterations    = 1000
	)

	var wg sync.WaitGroup

	for g := range numGoroutines {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()

			for i := range iterations {
				action := rand.IntN(6)
				switch action {
				case 0:
					// Pure nil context read
					var nilCtx context.Context
					if res := MetaFrom(nilCtx); res != nil {
						t.Errorf("g%d: MetaFrom(nil) != nil", gID)
					}
				case 1:
					// Background / TODO context without metadata
					if res := MetaFrom(context.Background()); res != nil {
						t.Errorf("g%d: MetaFrom(Background) != nil", gID)
					}
					if res := MetaFrom(context.TODO()); res != nil {
						t.Errorf("g%d: MetaFrom(TODO) != nil", gID)
					}
				case 2:
					// WithMeta with nil map
					ctx := WithMeta(context.Background(), nil)
					meta := MetaFrom(ctx)
					if meta == nil {
						t.Errorf("g%d: MetaFrom(WithMeta(nil)) is nil", gID)
					}
					if len(meta) != 0 {
						t.Errorf("g%d: MetaFrom(WithMeta(nil)) length = %d", gID, len(meta))
					}
				case 3:
					// WithMeta with real map & immediate local map mutation
					local := map[string]any{
						"id":        gID,
						"seq":       i,
						"immutable": true,
					}
					ctx := WithMeta(context.Background(), local)
					// Mutate local to ensure snapshot isolation
					local["immutable"] = false
					local["poison"] = "hacked"

					extracted := MetaFrom(ctx)
					if extracted == nil {
						t.Errorf("g%d: extracted is nil", gID)
					} else {
						if extracted["immutable"] != true {
							t.Errorf("g%d: snapshot isolation violated for immutable flag", gID)
						}
						if _, exists := extracted["poison"]; exists {
							t.Errorf("g%d: local mutation leaked into context metadata", gID)
						}
					}
				case 4:
					// Deep context chain with timeouts, cancels, values
					ctx0 := WithMeta(context.Background(), map[string]any{"root": gID})
					ctx1 := context.WithValue(ctx0, customTestCtxKey{}, "custom_val")
					ctx2, cancel := context.WithTimeout(ctx1, 100*time.Millisecond)
					ctx3 := WithMeta(ctx2, map[string]any{"leaf": i, "root": gID * 10})

					metaLeaf := MetaFrom(ctx3)
					if metaLeaf["leaf"] != i || metaLeaf["root"] != gID*10 {
						t.Errorf("g%d: metaLeaf mismatch: %v", gID, metaLeaf)
					}
					cancel()
				case 5:
					// Context with arbitrary non-metadata keys
					unrelatedCtx := context.WithValue(context.Background(), unrelatedCtxKey{}, 12345)
					if res := MetaFrom(unrelatedCtx); res != nil {
						t.Errorf("g%d: MetaFrom returned non-nil on unrelated ctx: %v", gID, res)
					}
				}
			}
		}(g)
	}

	wg.Wait()
	t.Logf("Adversarial MetaFrom stress test passed (%d goroutines x %d iterations)", numGoroutines, iterations)
}

// TestStressMetaFromAdversarial satisfies Objective 4:
// Adversarially test contextutil/metadata.MetaFrom with nil context, empty context,
// and concurrent context mutations.
func TestStressMetaFromAdversarial(t *testing.T) {
	// 1. Nil context
	var nilCtx context.Context
	if m := MetaFrom(nilCtx); m != nil {
		t.Fatalf("MetaFrom(nil) = %v, want nil", m)
	}

	// 2. Empty context / Context without metadata
	if m := MetaFrom(context.Background()); m != nil {
		t.Fatalf("MetaFrom(Background) = %v, want nil", m)
	}
	if m := MetaFrom(context.TODO()); m != nil {
		t.Fatalf("MetaFrom(TODO) = %v, want nil", m)
	}

	// 3. WithMeta with nil map and empty map
	ctxNilMap := WithMeta(context.Background(), nil)
	mNil := MetaFrom(ctxNilMap)
	if mNil == nil {
		t.Fatalf("MetaFrom(WithMeta(nil)) should return non-nil empty map, got nil")
	}
	if len(mNil) != 0 {
		t.Fatalf("MetaFrom(WithMeta(nil)) length = %d, want 0", len(mNil))
	}

	ctxEmptyMap := WithMeta(context.Background(), map[string]any{})
	mEmpty := MetaFrom(ctxEmptyMap)
	if mEmpty == nil {
		t.Fatalf("MetaFrom(WithMeta(empty)) should return non-nil empty map, got nil")
	}
	if len(mEmpty) != 0 {
		t.Fatalf("MetaFrom(WithMeta(empty)) length = %d, want 0", len(mEmpty))
	}

	// 4. Concurrent context creation, mutation, and reads across 100+ goroutines
	const numGoroutines = 100
	const iterations = 500

	var wg sync.WaitGroup
	baseCtx := context.Background()

	for g := range numGoroutines {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			for i := range iterations {
				// Local map that gets passed into WithMeta
				localMap := map[string]any{
					"worker": gID,
					"seq":    i,
					"tag":    "initial",
				}

				ctx := WithMeta(baseCtx, localMap)

				// Mutate original map immediately after WithMeta to verify snapshot isolation
				localMap["tag"] = "mutated"
				localMap["extra"] = "leak_attempt"

				// Read from context metadata
				meta := MetaFrom(ctx)
				if meta == nil {
					t.Errorf("goroutine %d: MetaFrom returned nil", gID)
					continue
				}

				if meta["worker"] != gID {
					t.Errorf("goroutine %d: meta[worker] = %v, want %d", gID, meta["worker"], gID)
				}
				if meta["seq"] != i {
					t.Errorf("goroutine %d: meta[seq] = %v, want %d", gID, meta["seq"], i)
				}
				if meta["tag"] != "initial" {
					t.Errorf("goroutine %d: meta[tag] = %v, want 'initial' (snapshot isolation violated)", gID, meta["tag"])
				}
				if _, exists := meta["extra"]; exists {
					t.Errorf("goroutine %d: meta has 'extra' key (mutated after WithMeta)", gID)
				}

				// Test MetaFrom with derived contexts
				childCtx, cancel := context.WithCancel(ctx)
				childMeta := MetaFrom(childCtx)
				if childMeta == nil || childMeta["worker"] != gID {
					t.Errorf("goroutine %d: childMeta mismatch", gID)
				}
				cancel()

				// Test nil context concurrently
				var nilC context.Context
				if MetaFrom(nilC) != nil {
					t.Errorf("goroutine %d: MetaFrom(nil) was non-nil", gID)
				}
			}
		}(g)
	}

	wg.Wait()
}
