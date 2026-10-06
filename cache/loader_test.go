package cache_test

import (
	"context"
	"encoding/json"
	"errors"
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

func TestGetExBuilderErrorNotCached(t *testing.T) {
	t.Parallel()

	c := newRecordingCache[string]()
	buildErr := errors.New("build failed")

	_, found, err := cache.GetEx(t.Context(), c, "k", func() (string, error) {
		return "partial", buildErr
	})
	if !errors.Is(err, buildErr) || found {
		t.Fatalf("GetEx = (found=%v, err=%v), want (false, %v)", found, err, buildErr)
	}
	if set, setTTL, _ := c.writes(); set != 0 || setTTL != 0 {
		t.Fatalf("writes = (%d, %d), want none after builder error", set, setTTL)
	}
	assertNotCached[string](t, c, "k")
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

func TestGetExNilBuilderOnMiss(t *testing.T) {
	t.Parallel()

	c := newRecordingCache[string]()
	val, found, err := cache.GetEx[string](t.Context(), c, "k", nil)
	if err != nil || found || val != "" {
		t.Fatalf("GetEx(nil builder) = (%q, %v, %v), want (\"\", false, nil)", val, found, err)
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

// TestGetExDynamicTTLExpires verifies the dynamic TTL actually reaches the
// driver by letting the fake clock run past it.
func TestGetExDynamicTTLExpires(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		c := mcache.NewMapCache[int]()
		opt := cache.WithDynamicTTL(func(v int) (bool, time.Duration) {
			return true, time.Duration(v) * time.Second
		})

		var calls int
		builder := func() (int, error) {
			calls++
			return 5, nil
		}
		for range 2 {
			if _, _, err := cache.GetEx(ctx, c, "k", builder, opt); err != nil {
				t.Fatalf("GetEx: %v", err)
			}
		}
		if calls != 1 {
			t.Fatalf("builder calls before expiry = %d, want 1", calls)
		}

		time.Sleep(6 * time.Second)
		if _, _, err := cache.GetEx(ctx, c, "k", builder, opt); err != nil {
			t.Fatalf("GetEx after expiry: %v", err)
		}
		if calls != 2 {
			t.Fatalf("builder calls after expiry = %d, want 2", calls)
		}
	})
}

func TestSetDynamicTTL(t *testing.T) {
	t.Parallel()

	c := newRecordingCache[int]()
	err := cache.Set(t.Context(), c, "k", 3, cache.WithDynamicTTL(func(v int) (bool, time.Duration) {
		return true, time.Duration(v) * time.Minute
	}))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, setTTL, ttl := c.writes(); setTTL != 1 || ttl != 3*time.Minute {
		t.Fatalf("SetWithTTL = (%d, %s), want (1, 3m)", setTTL, ttl)
	}

	err = cache.Set(t.Context(), c, "bad", 3, cache.WithDynamicTTL(func(string) (bool, time.Duration) {
		return true, time.Minute
	}))
	if !errors.Is(err, cache.ErrTTLCalculatorType) {
		t.Fatalf("Set mismatch err = %v, want ErrTTLCalculatorType", err)
	}
	assertNotCached[int](t, c, "bad")

	if err = cache.Set(t.Context(), c, "nil", 3, cache.WithDynamicTTL[int](nil)); !errors.Is(err, cache.ErrTTLCalculatorType) {
		t.Fatalf("Set nil calculator err = %v, want ErrTTLCalculatorType", err)
	}

	// WithNeverExpire after WithExpiration routes back to plain Set.
	before, _, _ := c.writes()
	if err = cache.Set(t.Context(), c, "never", 1, cache.WithExpiration(time.Second), cache.WithNeverExpire()); err != nil {
		t.Fatalf("Set never expire: %v", err)
	}
	if after, _, _ := c.writes(); after != before+1 {
		t.Fatalf("WithNeverExpire did not route to Set")
	}

	c.setErr = errors.New("write failed")
	if err = cache.Set(t.Context(), c, "fail", 1); !errors.Is(err, c.setErr) {
		t.Fatalf("Set backend error = %v, want %v", err, c.setErr)
	}
}

func TestSetJsonDynamicTTLSeesOriginalValue(t *testing.T) {
	t.Parallel()

	type payload struct {
		TTL time.Duration `json:"ttl"`
	}

	c := newRecordingCache[[]byte]()
	if err := cache.SetJson(t.Context(), c, "k", payload{TTL: 9 * time.Second},
		cache.WithDynamicTTL(func(p payload) (bool, time.Duration) { return true, p.TTL })); err != nil {
		t.Fatalf("SetJson: %v", err)
	}
	if _, setTTL, ttl := c.writes(); setTTL != 1 || ttl != 9*time.Second {
		t.Fatalf("SetWithTTL = (%d, %s), want (1, 9s)", setTTL, ttl)
	}
	raw, found, err := c.Get(t.Context(), "k")
	if err != nil || !found || string(raw) != `{"ttl":9000000000}` {
		t.Fatalf("stored = (%s, %v, %v)", raw, found, err)
	}

	// A calculator written against the encoded form does not match.
	err = cache.SetJson(t.Context(), c, "bad", payload{},
		cache.WithDynamicTTL(func([]byte) (bool, time.Duration) { return true, time.Second }))
	if !errors.Is(err, cache.ErrTTLCalculatorType) {
		t.Fatalf("SetJson mismatch err = %v, want ErrTTLCalculatorType", err)
	}
	assertNotCached[[]byte](t, c, "bad")

	if err = cache.SetJson(t.Context(), c, "unencodable", make(chan int)); err == nil {
		t.Fatalf("SetJson(chan) succeeded, want encode error")
	}
	assertNotCached[[]byte](t, c, "unencodable")
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

	buildErr := errors.New("build failed")
	if _, found, err := cache.GetJsonEx(ctx, c, "err", func() (payload, error) {
		return payload{}, buildErr
	}); !errors.Is(err, buildErr) || found {
		t.Fatalf("GetJsonEx builder error = (found=%v, err=%v)", found, err)
	}
	assertNotCached[[]byte](t, c, "err")
}

// TestGetJsonExCorruptEntry: an undecodable cached entry is a read error,
// not a miss — the builder is not consulted and the entry is left in place.
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

	called := false
	_, found, err := cache.GetJsonEx(ctx, c, "k", func() (payload, error) {
		called = true
		return payload{N: 1}, nil
	})
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) || found {
		t.Fatalf("GetJsonEx corrupt = (found=%v, err=%v), want json.SyntaxError", found, err)
	}
	if called {
		t.Fatalf("builder ran for an undecodable entry")
	}
	raw, ok, _ := c.Get(ctx, "k")
	if !ok || string(raw) != "not-json" {
		t.Fatalf("corrupt entry was modified: (%q, %v)", raw, ok)
	}
}

func TestGetObjectExCustomCodec(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	c := newRecordingCache[[]byte]()

	encode := codec.EncoderFunc(func(v any) ([]byte, error) {
		return []byte(v.(string) + "!"), nil
	})
	decodeErr := errors.New("decode failed")
	decode := codec.DecoderFunc(func(data []byte, v any) error {
		if len(data) == 0 || data[len(data)-1] != '!' {
			return decodeErr
		}
		*(v.(*string)) = string(data[:len(data)-1])
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

	got, found, err = cache.GetObjectEx[string](ctx, c, decode, encode, "k", func() (string, error) {
		t.Fatalf("builder ran on a hit")
		return "", nil
	})
	if err != nil || !found || got != "hello" {
		t.Fatalf("GetObjectEx hit = (%q, %v, %v)", got, found, err)
	}

	if err = c.Map.Set(ctx, "bad", []byte("nope")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, found, err = cache.GetObjectEx[string](ctx, c, decode, encode, "bad", func() (string, error) {
		return "x", nil
	}); !errors.Is(err, decodeErr) || found {
		t.Fatalf("GetObjectEx decode error = (found=%v, err=%v), want %v", found, err, decodeErr)
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
