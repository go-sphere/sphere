package cache_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/go-sphere/confstore/codec"
	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/mcache"
	"golang.org/x/sync/singleflight"
)

// recordingCache wraps an mcache driver and records which write path the
// loader helpers chose. Optional errors make the writes or reads fail.
type recordingCache[T any] struct {
	*mcache.Map[string, T]

	mu              sync.Mutex
	setCalls        int
	setWithTTLCalls int
	lastTTL         time.Duration

	setErr error
	getErr error
}

func newRecordingCache[T any]() *recordingCache[T] {
	return &recordingCache[T]{Map: mcache.NewMapCache[T]()}
}

func (c *recordingCache[T]) Set(ctx context.Context, key string, val T) error {
	c.mu.Lock()
	c.setCalls++
	err := c.setErr
	c.mu.Unlock()
	if err != nil {
		return err
	}
	return c.Map.Set(ctx, key, val)
}

func (c *recordingCache[T]) SetWithTTL(ctx context.Context, key string, val T, expiration time.Duration) error {
	c.mu.Lock()
	c.setWithTTLCalls++
	c.lastTTL = expiration
	err := c.setErr
	c.mu.Unlock()
	if err != nil {
		return err
	}
	return c.Map.SetWithTTL(ctx, key, val, expiration)
}

func (c *recordingCache[T]) Get(ctx context.Context, key string) (T, bool, error) {
	if c.getErr != nil {
		var zero T
		return zero, false, c.getErr
	}
	return c.Map.Get(ctx, key)
}

func (c *recordingCache[T]) writes() (set, setWithTTL int, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.setCalls, c.setWithTTLCalls, c.lastTTL
}

func assertNotCached[T any](t *testing.T, c cache.ExpirableCache[T], key string) {
	t.Helper()
	_, found, err := c.Get(t.Context(), key)
	if err != nil {
		t.Fatalf("Get(%q): %v", key, err)
	}
	if found {
		t.Fatalf("Get(%q) found an entry, want none", key)
	}
}

func TestGetExSingleflightNilInterfaceValue(t *testing.T) {
	c := mcache.NewMapCache[error]()
	builder := func() (error, error) { return nil, nil }
	v, found, err := cache.GetEx(t.Context(), c, "k", builder, cache.WithSingleflight(&singleflight.Group{}))
	if v != nil || !found || err != nil {
		t.Fatalf("GetEx = (%v, %v, %v), want (nil, true, nil)", v, found, err)
	}
}

// TestGetExSingleflightBuildsOnce holds the builder on a gate until every
// caller has joined the in-flight call, so the "exactly once" assertion does
// not depend on scheduler timing. synctest.Wait returns only once every
// goroutine in the bubble is durably blocked: one inside the builder on the
// gate, the rest waiting on the singleflight call.
func TestGetExSingleflightBuildsOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		c := mcache.NewMapCache[string]()
		group := &singleflight.Group{}
		gate := make(chan struct{})

		var calls atomic.Int32
		builder := func() (string, error) {
			calls.Add(1)
			<-gate
			return "shared", nil
		}

		const n = 8
		type result struct {
			val   string
			found bool
			err   error
		}
		results := make([]result, n)
		var wg sync.WaitGroup
		for i := range n {
			wg.Go(func() {
				v, found, err := cache.GetEx(ctx, c, "k", builder, cache.WithSingleflight(group))
				results[i] = result{v, found, err}
			})
		}

		synctest.Wait()
		if got := calls.Load(); got != 1 {
			t.Fatalf("builder calls while gated = %d, want 1", got)
		}
		close(gate)
		wg.Wait()

		if got := calls.Load(); got != 1 {
			t.Fatalf("builder calls = %d, want 1", got)
		}
		for i, r := range results {
			if r.err != nil || !r.found || r.val != "shared" {
				t.Fatalf("caller %d = (%q, %v, %v), want (\"shared\", true, nil)", i, r.val, r.found, r.err)
			}
		}
		val, found, err := c.Get(ctx, "k")
		if err != nil || !found || val != "shared" {
			t.Fatalf("backfilled entry = (%q, %v, %v), want (\"shared\", true, nil)", val, found, err)
		}
	})
}

// TestGetExSingleflightErrorSharedAndNotCached checks that every caller
// joined to a failing build receives the builder error and nothing is cached.
func TestGetExSingleflightErrorSharedAndNotCached(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		c := mcache.NewMapCache[int]()
		group := &singleflight.Group{}
		gate := make(chan struct{})
		buildErr := errors.New("build failed")

		var calls atomic.Int32
		builder := func() (int, error) {
			calls.Add(1)
			<-gate
			return 42, buildErr
		}

		const n = 4
		errs := make([]error, n)
		vals := make([]int, n)
		founds := make([]bool, n)
		var wg sync.WaitGroup
		for i := range n {
			wg.Go(func() {
				vals[i], founds[i], errs[i] = cache.GetEx(ctx, c, "k", builder, cache.WithSingleflight(group))
			})
		}
		synctest.Wait()
		close(gate)
		wg.Wait()

		if got := calls.Load(); got != 1 {
			t.Fatalf("builder calls = %d, want 1", got)
		}
		for i := range n {
			if !errors.Is(errs[i], buildErr) || founds[i] || vals[i] != 0 {
				t.Fatalf("caller %d = (%d, %v, %v), want (0, false, %v)", i, vals[i], founds[i], errs[i], buildErr)
			}
		}
		assertNotCached[int](t, c, "k")
	})
}

// TestGetExCacheWriteFailureStillReturnsValue pins the documented contract
// that a failed backfill does not fail the read: the built value is returned
// as found with a nil error.
func TestGetExCacheWriteFailureStillReturnsValue(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		opts []cache.Option
	}{
		{name: "plain"},
		{name: "singleflight", opts: []cache.Option{cache.WithSingleflight(&singleflight.Group{})}},
		{name: "ttl", opts: []cache.Option{cache.WithExpiration(time.Minute)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := newRecordingCache[string]()
			c.setErr = errors.New("backend down")

			val, found, err := cache.GetEx(t.Context(), c, "k", func() (string, error) {
				return "built", nil
			}, tc.opts...)
			if err != nil || !found || val != "built" {
				t.Fatalf("GetEx = (%q, %v, %v), want (\"built\", true, nil)", val, found, err)
			}
			if set, setTTL, _ := c.writes(); set+setTTL != 1 {
				t.Fatalf("write attempts = %d, want 1", set+setTTL)
			}
			assertNotCached[string](t, c, "k")
		})
	}
}

// TestGetExDynamicTTLTypeMismatchIgnoredOnRead shows how ErrTTLCalculatorType
// surfaces through the read-through path: the backfill is rejected (nothing
// stored), but per the GetEx contract the built value is still returned.
func TestGetExDynamicTTLTypeMismatchIgnoredOnRead(t *testing.T) {
	t.Parallel()

	c := newRecordingCache[int]()
	val, found, err := cache.GetEx(t.Context(), c, "k", func() (int, error) {
		return 7, nil
	}, cache.WithDynamicTTL(func(string) (bool, time.Duration) { return true, time.Minute }))
	if err != nil || !found || val != 7 {
		t.Fatalf("GetEx = (%d, %v, %v), want (7, true, nil)", val, found, err)
	}
	if set, setTTL, _ := c.writes(); set != 0 || setTTL != 0 {
		t.Fatalf("writes = (%d, %d), want none on calculator type mismatch", set, setTTL)
	}
	assertNotCached[int](t, c, "k")
}

func TestGetExGetterErrorSkipsBuilder(t *testing.T) {
	t.Parallel()

	c := newRecordingCache[string]()
	c.getErr = errors.New("read failed")

	called := false
	val, found, err := cache.GetEx(t.Context(), c, "k", func() (string, error) {
		called = true
		return "built", nil
	})
	if !errors.Is(err, c.getErr) || found || val != "" {
		t.Fatalf("GetEx = (%q, %v, %v), want (\"\", false, %v)", val, found, err, c.getErr)
	}
	if called {
		t.Fatalf("builder ran after a getter error")
	}
}

func TestGetExDynamicTTLApplied(t *testing.T) {
	t.Parallel()

	ttlFor := func(v int) (bool, time.Duration) {
		if v > 10 {
			return true, time.Duration(v) * time.Second
		}
		return false, 0
	}

	c := newRecordingCache[int]()
	if _, _, err := cache.GetEx(t.Context(), c, "long", func() (int, error) { return 30, nil },
		cache.WithDynamicTTL(ttlFor)); err != nil {
		t.Fatalf("GetEx long: %v", err)
	}
	if set, setTTL, ttl := c.writes(); set != 0 || setTTL != 1 || ttl != 30*time.Second {
		t.Fatalf("after long = (set=%d, setWithTTL=%d, ttl=%s), want (0, 1, 30s)", set, setTTL, ttl)
	}

	// The calculator overrides a static WithExpiration: returning false
	// stores without a TTL.
	if _, _, err := cache.GetEx(t.Context(), c, "short", func() (int, error) { return 1, nil },
		cache.WithExpiration(time.Hour), cache.WithDynamicTTL(ttlFor)); err != nil {
		t.Fatalf("GetEx short: %v", err)
	}
	if set, setTTL, _ := c.writes(); set != 1 || setTTL != 1 {
		t.Fatalf("after short = (set=%d, setWithTTL=%d), want (1, 1)", set, setTTL)
	}
}

// TestSetOptionOrderAndBackendError pins that the last TTL option wins and
// that Set, unlike the GetEx backfill, returns the backend's write error.
func TestSetOptionOrderAndBackendError(t *testing.T) {
	t.Parallel()

	c := newRecordingCache[int]()
	if err := cache.Set(t.Context(), c, "never", 1, cache.WithExpiration(time.Second), cache.WithNeverExpire()); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if set, setTTL, _ := c.writes(); set != 1 || setTTL != 0 {
		t.Fatalf("writes = (set=%d, setWithTTL=%d), want (1, 0)", set, setTTL)
	}

	c.setErr = errors.New("write failed")
	if err := cache.Set(t.Context(), c, "fail", 1); !errors.Is(err, c.setErr) {
		t.Fatalf("Set backend error = %v, want %v", err, c.setErr)
	}
}

func TestGetJsonExReadThrough(t *testing.T) {
	t.Parallel()

	type payload struct {
		N int `json:"n"`
	}

	ctx := t.Context()
	c := newRecordingCache[[]byte]()

	calls := 0
	builder := func() (payload, error) {
		calls++
		return payload{N: 1}, nil
	}
	for range 2 {
		got, found, err := cache.GetJsonEx(ctx, c, "k", builder, cache.WithExpiration(time.Minute))
		if err != nil || !found || got.N != 1 {
			t.Fatalf("GetJsonEx = (%+v, %v, %v), want ({1}, true, nil)", got, found, err)
		}
	}
	if calls != 1 {
		t.Fatalf("builder calls = %d, want 1 (second read should hit)", calls)
	}
	if _, setTTL, ttl := c.writes(); setTTL != 1 || ttl != time.Minute {
		t.Fatalf("backfill = (%d, %s), want (1, 1m)", setTTL, ttl)
	}
	raw, _, _ := c.Get(ctx, "k")
	if string(raw) != `{"n":1}` {
		t.Fatalf("stored bytes = %s, want JSON", raw)
	}
}

// TestGetJsonExCorruptEntry: an undecodable cached entry is a miss for the
// read-through loader — the builder runs and its result overwrites the entry —
// while the plain GetJson read reports ErrDecode and the codec error.
func TestGetJsonExCorruptEntry(t *testing.T) {
	t.Parallel()

	type payload struct {
		N int `json:"n"`
	}

	ctx := t.Context()
	c := mcache.NewByteCache()
	if err := c.Set(ctx, "k", []byte("not-json")); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, found, err := cache.GetJson[payload](ctx, c, "k")
	if _, ok := errors.AsType[*json.SyntaxError](err); !ok || !errors.Is(err, cache.ErrDecode) || found {
		t.Fatalf("GetJson corrupt = (found=%v, err=%v), want ErrDecode wrapping json.SyntaxError", found, err)
	}

	calls := 0
	got, found, err := cache.GetJsonEx(ctx, c, "k", func() (payload, error) {
		calls++
		return payload{N: 1}, nil
	})
	if err != nil || !found || got.N != 1 || calls != 1 {
		t.Fatalf("GetJsonEx corrupt = (%v, %v, %v), calls=%d, want rebuilt value", got, found, err, calls)
	}
	raw, ok, _ := c.Get(ctx, "k")
	if !ok || string(raw) != `{"n":1}` {
		t.Fatalf("entry not overwritten: (%q, %v)", raw, ok)
	}
}

// TestGetJsonExSchemaChange: an entry written under an older schema whose
// field type changed is rebuilt once and then served from the cache.
func TestGetJsonExSchemaChange(t *testing.T) {
	t.Parallel()

	type v1 struct {
		ID string `json:"id"`
	}
	type v2 struct {
		ID int64 `json:"id"`
	}

	ctx := t.Context()
	c := mcache.NewByteCache()
	if err := cache.SetJson(ctx, c, "user:1", v1{ID: "abc"}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	calls := 0
	builder := func() (v2, error) {
		calls++
		return v2{ID: 7}, nil
	}
	for i := range 3 {
		got, found, err := cache.GetJsonEx(ctx, c, "user:1", builder)
		if err != nil || !found || got.ID != 7 {
			t.Fatalf("read %d = (%v, %v, %v), want ({7}, true, nil)", i, got, found, err)
		}
	}
	if calls != 1 {
		t.Fatalf("builder calls = %d, want 1", calls)
	}
}

// TestGetExCodecCacheDecodeErrorIsMiss: GetEx over a CodecCache rebuilds an
// undecodable entry, since CodecCache.Get reports it as ErrDecode.
func TestGetExCodecCacheDecodeErrorIsMiss(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	inner := mcache.NewByteCache()
	if err := inner.Set(ctx, "k", []byte("{bad")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	typed := cache.NewJsonCache[int](inner)
	got, found, err := cache.GetEx(ctx, typed, "k", func() (int, error) { return 5, nil })
	if err != nil || !found || got != 5 {
		t.Fatalf("GetEx = (%d, %v, %v), want (5, true, nil)", got, found, err)
	}
	if raw, _, _ := inner.Get(ctx, "k"); string(raw) != "5" {
		t.Fatalf("entry = %q, want 5", raw)
	}
}

// TestGetJsonExNilBuilderDecodeError: with no builder an undecodable entry
// cannot be rebuilt, so the decode error is returned and the entry is kept.
func TestGetJsonExNilBuilderDecodeError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	c := mcache.NewByteCache()
	if err := c.Set(ctx, "k", []byte("{bad")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, found, err := cache.GetJsonEx[int](ctx, c, "k", nil)
	if !errors.Is(err, cache.ErrDecode) || found || got != 0 {
		t.Fatalf("GetJsonEx nil builder = (%d, %v, %v), want (0, false, ErrDecode)", got, found, err)
	}
	if raw, ok, _ := c.Get(ctx, "k"); !ok || string(raw) != "{bad" {
		t.Fatalf("entry = (%q, %v), want untouched", raw, ok)
	}
}

func TestGetObjectExCustomCodec(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	c := newRecordingCache[[]byte]()

	encode := codec.EncoderFunc(func(v any) ([]byte, error) {
		return []byte(v.(string) + "!"), nil
	})
	decode := codec.DecoderFunc(func(data []byte, v any) error {
		*(v.(*string)) = strings.TrimSuffix(string(data), "!")
		return nil
	})

	got, found, err := cache.GetObjectEx[string](ctx, c, decode, encode, "k", func() (string, error) {
		return "hello", nil
	})
	if err != nil || !found || got != "hello" {
		t.Fatalf("GetObjectEx miss = (%q, %v, %v)", got, found, err)
	}
	raw, _, _ := c.Get(ctx, "k")
	if string(raw) != "hello!" {
		t.Fatalf("stored = %q, want encoder output", raw)
	}

	// An encoder failure on backfill is a setter error: ignored by the loader.
	encodeErr := errors.New("encode failed")
	failingEncode := codec.EncoderFunc(func(any) ([]byte, error) { return nil, encodeErr })
	got, found, err = cache.GetObjectEx[string](ctx, c, decode, failingEncode, "enc", func() (string, error) {
		return "v", nil
	})
	if err != nil || !found || got != "v" {
		t.Fatalf("GetObjectEx encode failure = (%q, %v, %v), want (\"v\", true, nil)", got, found, err)
	}
	assertNotCached[[]byte](t, c, "enc")
}
