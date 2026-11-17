package test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/mcache"
	"github.com/go-sphere/sphere/cache/nscache"
)

func TestKeyListerContract(t *testing.T) {
	t.Parallel()

	for _, factory := range statefulByteCacheFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			c := factory.new(t)
			lister, ok := c.(cache.KeyLister)
			if !ok {
				t.Skipf("%s does not implement cache.KeyLister", factory.name)
			}

			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			if _, err := lister.Keys(cancelled, ""); !errors.Is(err, context.Canceled) {
				t.Fatalf("Keys(canceled) = %v, want context.Canceled", err)
			}

			entries := map[string][]byte{
				"user:1":   []byte("a"),
				"user:2":   []byte("b"),
				"post:1":   []byte("c"),
				"glob*?[1": []byte("d"),
			}
			if err := c.MultiSet(ctx, entries); err != nil {
				t.Fatalf("MultiSet: %v", err)
			}

			all, err := lister.Keys(ctx, "")
			if err != nil {
				t.Fatalf("Keys(\"\"): %v", err)
			}
			slices.Sort(all)
			wantAll := []string{"glob*?[1", "post:1", "user:1", "user:2"}
			if !slices.Equal(all, wantAll) {
				t.Fatalf("Keys(\"\") = %v, want %v", all, wantAll)
			}

			users, err := lister.Keys(ctx, "user:")
			if err != nil {
				t.Fatalf("Keys(\"user:\"): %v", err)
			}
			slices.Sort(users)
			if !slices.Equal(users, []string{"user:1", "user:2"}) {
				t.Fatalf("Keys(\"user:\") = %v, want [user:1 user:2]", users)
			}

			literal, err := lister.Keys(ctx, "glob*?[")
			if err != nil {
				t.Fatalf("Keys(glob literal): %v", err)
			}
			if !slices.Equal(literal, []string{"glob*?[1"}) {
				t.Fatalf("Keys(glob literal) = %v, want [glob*?[1]", literal)
			}
		})
	}
}

func TestNoCacheKeyListerCanceledContext(t *testing.T) {
	t.Parallel()

	c := noCacheFactory().new(t)
	lister, ok := c.(cache.KeyLister)
	if !ok {
		t.Fatal("nocache should implement cache.KeyLister")
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := lister.Keys(ctx, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("nocache Keys(canceled) = %v, want context.Canceled", err)
	}
}

func TestNSCacheDelAllLargeNamespaceAndCanceledContext(t *testing.T) {
	t.Parallel()

	backend := mcache.NewByteCache()
	t.Cleanup(func() { _ = backend.Close() })

	ns1 := nscache.NewNSCache[[]byte]("ns1", backend)
	ns2 := nscache.NewNSCache[[]byte]("ns2", backend)

	ctx := t.Context()
	batch := make(map[string][]byte, 1205)
	for i := range 1205 {
		batch[fmt.Sprintf("k%d", i)] = []byte("v")
	}
	if err := ns1.MultiSet(ctx, batch); err != nil {
		t.Fatalf("ns1.MultiSet: %v", err)
	}
	if err := ns2.Set(ctx, "keep", []byte("1")); err != nil {
		t.Fatalf("ns2.Set: %v", err)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := ns1.DelAll(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("ns1.DelAll(canceled) = %v, want context.Canceled", err)
	}

	if err := ns1.DelAll(ctx); err != nil {
		t.Fatalf("ns1.DelAll: %v", err)
	}
	remaining, err := ns1.Keys(ctx, "")
	if err != nil || len(remaining) != 0 {
		t.Fatalf("ns1.Keys after DelAll = (%v, %v), want empty", remaining, err)
	}
	if ok, err := ns2.Exists(ctx, "keep"); err != nil || !ok {
		t.Fatalf("ns2.Exists(keep) = (%v, %v), want (true, nil)", ok, err)
	}
}
