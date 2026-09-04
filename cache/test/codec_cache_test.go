package test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-sphere/confstore/codec"
	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/badgerdb"
	"github.com/go-sphere/sphere/cache/mcache"
	"github.com/go-sphere/sphere/cache/nscache"
)

func TestCodecCacheImplementsContract(t *testing.T) {
	t.Parallel()

	typed := cache.NewCodecCache[string](mcache.NewByteCache(), codec.JsonCodec())

	if typed.GetByteCache() == nil {
		t.Fatalf("GetByteCache returned nil")
	}
	if typed.GetCodec() == nil {
		t.Fatalf("GetCodec returned nil")
	}

	var _ cache.Cache[string] = typed
}

// TestWrapperCloseLeavesBackendOpen pins the ownership rule at the wrapper
// layer: CodecCache and NSCache only ever receive an injected cache, so
// closing one must leave the backend usable for its owner and for any sibling
// wrapper. badgerdb is used because its Close is observable — mcache.Close is
// already a no-op and could not detect a regression here.
func TestWrapperCloseLeavesBackendOpen(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	backend, err := badgerdb.NewDatabase(badgerdb.Config{Path: t.TempDir()})
	if err != nil {
		t.Fatalf("create badgerdb: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	typed := cache.NewJsonCache[string](backend)
	ns := nscache.NewNSCache[[]byte]("ns", backend)

	if err := backend.Set(ctx, "k", []byte("v")); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := typed.Close(); err != nil {
		t.Fatalf("CodecCache.Close: %v", err)
	}
	if err := ns.Close(); err != nil {
		t.Fatalf("NSCache.Close: %v", err)
	}

	val, found, err := backend.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get after wrapper Close: %v", err)
	}
	if !found || string(val) != "v" {
		t.Fatalf("wrapper Close closed the injected backend: found=%v value=%q", found, string(val))
	}
}

// TestCodecCacheTTLAndDelete exercises the adapter methods that the encode /
// decode round trip does not cover on its own: the TTL writes and the three
// delete paths.
func TestCodecCacheTTLAndDelete(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	typed := cache.NewJsonCache[int](mcache.NewByteCache())

	if err := typed.SetWithTTL(ctx, "a", 1, time.Minute); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}
	if err := typed.MultiSetWithTTL(ctx, map[string]int{"b": 2, "c": 3}, time.Minute); err != nil {
		t.Fatalf("MultiSetWithTTL: %v", err)
	}

	got, found, err := typed.GetDel(ctx, "a")
	if err != nil {
		t.Fatalf("GetDel: %v", err)
	}
	if !found || got != 1 {
		t.Fatalf("GetDel mismatch: found=%v got=%d", found, got)
	}
	if found, err := typed.Exists(ctx, "a"); err != nil {
		t.Fatalf("Exists after GetDel: %v", err)
	} else if found {
		t.Fatalf("GetDel must remove the key")
	}

	if err := typed.Del(ctx, "b"); err != nil {
		t.Fatalf("Del: %v", err)
	}
	if found, err := typed.Exists(ctx, "b"); err != nil {
		t.Fatalf("Exists after Del: %v", err)
	} else if found {
		t.Fatalf("Del must remove the key")
	}

	if err := typed.DelAll(ctx); err != nil {
		t.Fatalf("DelAll: %v", err)
	}
	if found, err := typed.Exists(ctx, "c"); err != nil {
		t.Fatalf("Exists after DelAll: %v", err)
	} else if found {
		t.Fatalf("DelAll must remove every key")
	}
}

func TestCodecCacheGetDelUnmarshalErrorConsumesKey(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	inner := mcache.NewByteCache()
	typed := cache.NewJsonCache[int](inner)
	if err := inner.Set(ctx, "k", []byte("not-json")); err != nil {
		t.Fatalf("Set raw: %v", err)
	}

	_, found, err := typed.GetDel(ctx, "k")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
	if !found {
		t.Fatal("found should be true because GetDel consumed an existing entry")
	}
	exists, existsErr := inner.Exists(ctx, "k")
	if existsErr != nil {
		t.Fatalf("Exists: %v", existsErr)
	}
	if exists {
		t.Fatal("undecodable key must be consumed by GetDel")
	}
}

type getDelSpyByteCache struct {
	cache.ByteCache
	getCalls    int
	getDelCalls int
}

func (c *getDelSpyByteCache) Get(context.Context, string) ([]byte, bool, error) {
	c.getCalls++
	return nil, false, errors.New("CodecCache.GetDel must not call Get")
}

func (c *getDelSpyByteCache) GetDel(ctx context.Context, key string) ([]byte, bool, error) {
	c.getDelCalls++
	return c.ByteCache.GetDel(ctx, key)
}

func TestCodecCacheGetDelDelegatesToAtomicPrimitive(t *testing.T) {
	t.Parallel()

	inner := &getDelSpyByteCache{
		ByteCache: mcache.NewByteCache(),
	}
	typed := cache.NewJsonCache[int](inner)
	if err := typed.Set(t.Context(), "once", 42); err != nil {
		t.Fatalf("Set: %v", err)
	}

	value, found, err := typed.GetDel(t.Context(), "once")
	if err != nil {
		t.Fatalf("GetDel: %v", err)
	}
	if !found || value != 42 {
		t.Fatalf("GetDel = (%d, %v), want (42, true)", value, found)
	}
	if inner.getCalls != 0 || inner.getDelCalls != 1 {
		t.Fatalf("backend calls = Get:%d GetDel:%d, want Get:0 GetDel:1", inner.getCalls, inner.getDelCalls)
	}
}

func TestJsonCacheAdapter(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	byteCache := mcache.NewByteCache()
	typed := cache.NewJsonCache[map[string]int](byteCache)

	if typed.GetByteCache() == nil {
		t.Fatalf("GetByteCache returned nil")
	}
	if typed.GetCodec() == nil {
		t.Fatalf("GetCodec returned nil")
	}

	if err := typed.Set(ctx, "k", map[string]int{"v": 1}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, found, err := typed.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found || got["v"] != 1 {
		t.Fatalf("Get mismatch: found=%v got=%v", found, got)
	}

	if err := typed.MultiSet(ctx, map[string]map[string]int{
		"k2": {"v": 2},
		"k3": {"v": 3},
	}); err != nil {
		t.Fatalf("MultiSet: %v", err)
	}
	m, err := typed.MultiGet(ctx, []string{"k2", "k3"})
	if err != nil {
		t.Fatalf("MultiGet: %v", err)
	}
	if m["k2"]["v"] != 2 || m["k3"]["v"] != 3 {
		t.Fatalf("MultiGet mismatch: %v", m)
	}
}

func TestCodecCacheMultiGetDoesNotDeleteConcurrentRepair(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	inner := mcache.NewByteCache()
	started := make(chan struct{})
	resume := make(chan struct{})
	jsonCodec := codec.JsonCodec()
	blockingCodec := codec.NewCodec(jsonCodec.Marshal, func(data []byte, value any) error {
		if string(data) == "broken" {
			close(started)
			<-resume
			return errors.New("cannot decode stale value")
		}
		return jsonCodec.Unmarshal(data, value)
	})
	typed := cache.NewCodecCache[int](inner, blockingCodec)
	if err := inner.Set(ctx, "k", []byte("broken")); err != nil {
		t.Fatalf("seed stale value: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := typed.MultiGet(ctx, []string{"k"})
		done <- err
	}()
	<-started
	if err := typed.Set(ctx, "k", 42); err != nil {
		t.Fatalf("repair value: %v", err)
	}
	close(resume)
	if err := <-done; err != nil {
		t.Fatalf("MultiGet: %v", err)
	}

	got, found, err := typed.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get repaired value: %v", err)
	}
	if !found || got != 42 {
		t.Fatalf("concurrent repair was deleted: found=%v got=%d", found, got)
	}
}
