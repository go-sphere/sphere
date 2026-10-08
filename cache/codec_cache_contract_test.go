package cache_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/mcache"
	"github.com/go-sphere/sphere/cache/nocache"
)

// plainByteCache hides the backend's optional KeyLister: only the Cache
// interface methods are promoted.
type plainByteCache struct {
	cache.ByteCache
}

// failingReadByteCache fails every read with err.
type failingReadByteCache struct {
	cache.ByteCache
	err error
}

func (c *failingReadByteCache) Get(context.Context, string) ([]byte, bool, error) {
	return nil, false, c.err
}

func (c *failingReadByteCache) GetDel(context.Context, string) ([]byte, bool, error) {
	return nil, false, c.err
}

func (c *failingReadByteCache) MultiGet(context.Context, []string) (map[string][]byte, error) {
	return nil, c.err
}

func seedRaw(t *testing.T, c cache.ByteCache, entries map[string]string) {
	t.Helper()
	for k, v := range entries {
		if err := c.Set(t.Context(), k, []byte(v)); err != nil {
			t.Fatalf("seed %q: %v", k, err)
		}
	}
}

func assertExists(t *testing.T, c interface {
	Exists(context.Context, string) (bool, error)
}, key string, want bool) {
	t.Helper()
	got, err := c.Exists(t.Context(), key)
	if err != nil {
		t.Fatalf("Exists(%q): %v", key, err)
	}
	if got != want {
		t.Fatalf("Exists(%q) = %v, want %v", key, got, want)
	}
}

// TestCodecCacheCorruptEntry contrasts the single-key and batch read paths
// for an undecodable entry: Get reports the decode error and leaves the entry
// in place, MultiGet omits it without failing the batch or deleting it.
func TestCodecCacheCorruptEntry(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	inner := mcache.NewByteCache()
	typed := cache.NewJsonCache[int](inner)
	seedRaw(t, inner, map[string]string{"good": "1", "bad": "{not-json", "also": "3"})

	val, found, err := typed.Get(ctx, "bad")
	if !errors.Is(err, cache.ErrDecode) || found || val != 0 {
		t.Fatalf("Get(bad) = (%d, %v, %v), want (0, false, ErrDecode)", val, found, err)
	}
	assertExists(t, inner, "bad", true)

	got, err := typed.MultiGet(ctx, []string{"good", "bad", "missing", "also"})
	if err != nil {
		t.Fatalf("MultiGet: %v", err)
	}
	if len(got) != 2 || got["good"] != 1 || got["also"] != 3 {
		t.Fatalf("MultiGet = %v, want map[also:3 good:1]", got)
	}
	if _, ok := got["bad"]; ok {
		t.Fatalf("MultiGet returned the undecodable entry")
	}
	assertExists(t, inner, "bad", true)

	val, found, err = typed.Get(ctx, "missing")
	if err != nil || found || val != 0 {
		t.Fatalf("Get(missing) = (%d, %v, %v), want (0, false, nil)", val, found, err)
	}
	val, found, err = typed.GetDel(ctx, "missing")
	if err != nil || found || val != 0 {
		t.Fatalf("GetDel(missing) = (%d, %v, %v), want (0, false, nil)", val, found, err)
	}
}

func TestCodecCacheBackendReadErrors(t *testing.T) {
	t.Parallel()

	readErr := errors.New("backend read failed")
	typed := cache.NewJsonCache[int](&failingReadByteCache{ByteCache: mcache.NewByteCache(), err: readErr})

	if _, found, err := typed.Get(t.Context(), "k"); !errors.Is(err, readErr) || found {
		t.Fatalf("Get = (found=%v, err=%v), want (false, %v)", found, err, readErr)
	}
	if _, found, err := typed.GetDel(t.Context(), "k"); !errors.Is(err, readErr) || found {
		t.Fatalf("GetDel = (found=%v, err=%v), want (false, %v)", found, err, readErr)
	}
	if got, err := typed.MultiGet(t.Context(), []string{"k"}); !errors.Is(err, readErr) || got != nil {
		t.Fatalf("MultiGet = (%v, %v), want (nil, %v)", got, err, readErr)
	}
}

// TestCodecCacheBulkDelete covers MultiDel and DelAll, which forwards to the
// backend and so also clears keys the adapter never wrote.
func TestCodecCacheBulkDelete(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	inner := mcache.NewByteCache()
	typed := cache.NewJsonCache[int](inner)
	if err := typed.MultiSet(ctx, map[string]int{"b": 2, "c": 3, "d": 4}); err != nil {
		t.Fatalf("MultiSet: %v", err)
	}

	if err := typed.MultiDel(ctx, []string{"b", "c", "missing"}); err != nil {
		t.Fatalf("MultiDel: %v", err)
	}
	assertExists(t, inner, "b", false)
	assertExists(t, inner, "c", false)
	assertExists(t, inner, "d", true)

	seedRaw(t, inner, map[string]string{"raw": "x"})
	if err := typed.DelAll(ctx); err != nil {
		t.Fatalf("DelAll: %v", err)
	}
	keys, err := inner.Keys(ctx, "")
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("keys after DelAll = %v, want none", keys)
	}
}

func TestCodecCacheKeys(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	inner := mcache.NewByteCache()
	typed := cache.NewJsonCache[int](inner)
	if err := typed.MultiSet(ctx, map[string]int{"user:1": 1, "user:2": 2, "post:1": 3}); err != nil {
		t.Fatalf("MultiSet: %v", err)
	}

	var _ cache.KeyLister = typed
	keys, err := typed.Keys(ctx, "user:")
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	slices.Sort(keys)
	if !slices.Equal(keys, []string{"user:1", "user:2"}) {
		t.Fatalf("Keys(user:) = %v, want [user:1 user:2]", keys)
	}
	all, err := typed.Keys(ctx, "")
	if err != nil || len(all) != 3 {
		t.Fatalf("Keys(\"\") = (%v, %v), want 3 keys", all, err)
	}

	empty, err := cache.NewJsonCache[int](nocache.NewNoCache[[]byte]()).Keys(ctx, "")
	if err != nil || len(empty) != 0 {
		t.Fatalf("Keys over nocache = (%v, %v), want (empty, nil)", empty, err)
	}

	unlisted := cache.NewJsonCache[int](plainByteCache{ByteCache: inner})
	if keys, err := unlisted.Keys(ctx, ""); !errors.Is(err, cache.ErrNotSupported) || keys != nil {
		t.Fatalf("Keys without KeyLister = (%v, %v), want (nil, ErrNotSupported)", keys, err)
	}
}
