package test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sphere/confstore/codec"
	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/mcache"
	"golang.org/x/sync/singleflight"
)

func TestIsZero(t *testing.T) {
	t.Parallel()

	type sample struct {
		A int
		B string
	}

	if !cache.IsZero(0) {
		t.Fatalf("expected zero int")
	}
	if cache.IsZero(1) {
		t.Fatalf("expected non-zero int")
	}
	if !cache.IsZero(sample{}) {
		t.Fatalf("expected zero struct")
	}
	if cache.IsZero(sample{A: 1}) {
		t.Fatalf("expected non-zero struct")
	}
}

func TestSetOptions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	spy := &spyExpirableCache[int]{}

	if err := cache.Set(ctx, spy, "k0", 1); err != nil {
		t.Fatalf("Set default: %v", err)
	}
	if spy.setCalls != 1 || spy.setWithTTLCalls != 0 {
		t.Fatalf("default Set route mismatch: set=%d setWithTTL=%d", spy.setCalls, spy.setWithTTLCalls)
	}

	if err := cache.Set(ctx, spy, "k1", 2, cache.WithExpiration(time.Second)); err != nil {
		t.Fatalf("Set WithExpiration: %v", err)
	}
	if spy.setWithTTLCalls != 1 || spy.lastTTL != time.Second {
		t.Fatalf("WithExpiration route mismatch: setWithTTL=%d ttl=%s", spy.setWithTTLCalls, spy.lastTTL)
	}

	if err := cache.Set(ctx, spy, "k2", 3, cache.WithNeverExpire()); err != nil {
		t.Fatalf("Set WithNeverExpire: %v", err)
	}
	if spy.setCalls != 2 {
		t.Fatalf("WithNeverExpire route mismatch: set=%d", spy.setCalls)
	}

	if err := cache.Set(ctx, spy, "k3", 7, cache.WithDynamicTTL(func(v int) (bool, time.Duration) {
		if v > 5 {
			return true, 10 * time.Millisecond
		}
		return false, 0
	})); err != nil {
		t.Fatalf("Set WithDynamicTTL true branch: %v", err)
	}
	if spy.setWithTTLCalls != 2 || spy.lastTTL != 10*time.Millisecond {
		t.Fatalf("WithDynamicTTL true branch mismatch: setWithTTL=%d ttl=%s", spy.setWithTTLCalls, spy.lastTTL)
	}

	if err := cache.Set(ctx, spy, "k4", 1, cache.WithDynamicTTL(func(v int) (bool, time.Duration) {
		if v > 5 {
			return true, time.Second
		}
		return false, 0
	})); err != nil {
		t.Fatalf("Set WithDynamicTTL false branch: %v", err)
	}
	if spy.setCalls != 3 {
		t.Fatalf("WithDynamicTTL false branch mismatch: set=%d", spy.setCalls)
	}
}

func TestSetObjectAndGetObject(t *testing.T) {
	t.Parallel()

	type payload struct {
		Value string `json:"value"`
	}

	ctx := context.Background()
	c := mcache.NewMapCache[[]byte]()

	want := payload{Value: "ok"}
	if err := cache.SetObject[payload, codec.EncoderFunc](ctx, c, json.Marshal, "obj", want); err != nil {
		t.Fatalf("SetObject: %v", err)
	}
	got, found, err := cache.GetObject[payload, codec.DecoderFunc](ctx, c, json.Unmarshal, "obj")
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	if !found || got != want {
		t.Fatalf("GetObject mismatch: found=%v got=%+v want=%+v", found, got, want)
	}

	_, found, err = cache.GetObject[payload, codec.DecoderFunc](ctx, c, json.Unmarshal, "missing")
	if err != nil {
		t.Fatalf("GetObject missing: %v", err)
	}
	if found {
		t.Fatalf("GetObject missing mismatch: expected not found")
	}

	encErr := errors.New("encode")
	err = cache.SetObject[payload, codec.EncoderFunc](ctx, c, codec.EncoderFunc(func(v any) ([]byte, error) {
		return nil, encErr
	}), "obj_err", want)
	if !errors.Is(err, encErr) {
		t.Fatalf("SetObject encode error mismatch: %v", err)
	}

	if err = c.Set(ctx, "broken", []byte("not-json")); err != nil {
		t.Fatalf("set broken payload: %v", err)
	}
	_, _, err = cache.GetObject[payload, codec.DecoderFunc](ctx, c, json.Unmarshal, "broken")
	if err == nil {
		t.Fatalf("expected GetObject decode error")
	}
}

func TestSetJsonAndGetJson(t *testing.T) {
	t.Parallel()

	type payload struct {
		N int `json:"n"`
	}

	ctx := context.Background()
	c := mcache.NewMapCache[[]byte]()

	if err := cache.SetJson(ctx, c, "json", payload{N: 42}); err != nil {
		t.Fatalf("SetJson: %v", err)
	}
	got, found, err := cache.GetJson[payload](ctx, c, "json")
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	if !found || got.N != 42 {
		t.Fatalf("GetJson mismatch: found=%v got=%+v", found, got)
	}
}

func TestGetEx(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	c := mcache.NewMapCache[string]()

	if err := c.Set(ctx, "hit", "value"); err != nil {
		t.Fatalf("seed hit key: %v", err)
	}
	builderCalls := 0
	val, found, err := cache.GetEx(ctx, c, "hit", func() (string, error) {
		builderCalls++
		return "new", nil
	})
	if err != nil {
		t.Fatalf("GetEx hit: %v", err)
	}
	if !found || val != "value" || builderCalls != 0 {
		t.Fatalf("GetEx hit mismatch: found=%v val=%q builderCalls=%d", found, val, builderCalls)
	}

	builderCalls = 0
	val, found, err = cache.GetEx(ctx, c, "miss", func() (string, error) {
		builderCalls++
		return "built", nil
	})
	if err != nil {
		t.Fatalf("GetEx miss with builder: %v", err)
	}
	if !found || val != "built" || builderCalls != 1 {
		t.Fatalf("GetEx miss mismatch: found=%v val=%q builderCalls=%d", found, val, builderCalls)
	}

	cached, ok, err := c.Get(ctx, "miss")
	if err != nil {
		t.Fatalf("read back built value: %v", err)
	}
	if !ok || cached != "built" {
		t.Fatalf("cache backfill mismatch: found=%v val=%q", ok, cached)
	}

	bErr := errors.New("builder")
	val, found, err = cache.GetEx(ctx, c, "err", func() (string, error) {
		return "", bErr
	})
	if !errors.Is(err, bErr) || found || val != "" {
		t.Fatalf("GetEx builder error mismatch: found=%v val=%q err=%v", found, val, err)
	}

	_, found, err = c.Get(ctx, "err")
	if err != nil {
		t.Fatalf("read after builder error: %v", err)
	}
	if found {
		t.Fatalf("cache should not backfill when builder errors")
	}

	val, found, err = cache.GetEx[string](ctx, c, "nil-builder", nil)
	if err != nil || found || val != "" {
		t.Fatalf("GetEx nil builder mismatch: found=%v val=%q err=%v", found, val, err)
	}
}

func TestGetExSingleflight(t *testing.T) {
	ctx := context.Background()
	c := mcache.NewMapCache[string]()
	g := &singleflight.Group{}

	var calls atomic.Int32
	builder := func() (string, error) {
		calls.Add(1)
		time.Sleep(20 * time.Millisecond)
		return "shared", nil
	}

	const n = 16
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, found, err := cache.GetEx(ctx, c, "singleflight", builder, cache.WithSingleflight(g))
			if err != nil {
				errCh <- err
				return
			}
			if !found || v != "shared" {
				errCh <- errors.New("value mismatch")
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("GetEx singleflight: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("singleflight builder call mismatch: got=%d want=1", calls.Load())
	}
}

func TestGetObjectExAndGetJsonEx(t *testing.T) {
	t.Parallel()

	type payload struct {
		Value string `json:"value"`
	}

	ctx := context.Background()
	c := mcache.NewMapCache[[]byte]()

	if err := cache.SetJson(ctx, c, "hit", payload{Value: "cached"}); err != nil {
		t.Fatalf("seed json: %v", err)
	}

	builderCalls := 0
	got, found, err := cache.GetObjectEx[payload, codec.DecoderFunc, codec.EncoderFunc](
		ctx,
		c,
		json.Unmarshal,
		json.Marshal,
		"hit",
		func() (payload, error) {
			builderCalls++
			return payload{Value: "new"}, nil
		},
	)
	if err != nil {
		t.Fatalf("GetObjectEx hit: %v", err)
	}
	if !found || got.Value != "cached" || builderCalls != 0 {
		t.Fatalf("GetObjectEx hit mismatch: found=%v got=%+v builderCalls=%d", found, got, builderCalls)
	}

	got, found, err = cache.GetJsonEx(ctx, c, "miss", func() (payload, error) {
		return payload{Value: "built"}, nil
	})
	if err != nil {
		t.Fatalf("GetJsonEx miss: %v", err)
	}
	if !found || got.Value != "built" {
		t.Fatalf("GetJsonEx miss mismatch: found=%v got=%+v", found, got)
	}

	_, found, err = cache.GetJsonEx[payload](ctx, c, "nil", nil)
	if err != nil {
		t.Fatalf("GetJsonEx nil builder: %v", err)
	}
	if found {
		t.Fatalf("GetJsonEx nil builder mismatch: expected not found")
	}
}

type spyExpirableCache[T any] struct {
	store           map[string]T
	setCalls        int
	setWithTTLCalls int
	lastTTL         time.Duration
}

func (s *spyExpirableCache[T]) Set(ctx context.Context, key string, val T) error {
	if s.store == nil {
		s.store = make(map[string]T)
	}
	s.setCalls++
	s.store[key] = val
	return nil
}

func (s *spyExpirableCache[T]) SetWithTTL(ctx context.Context, key string, val T, expiration time.Duration) error {
	if s.store == nil {
		s.store = make(map[string]T)
	}
	s.setWithTTLCalls++
	s.lastTTL = expiration
	s.store[key] = val
	return nil
}

func (s *spyExpirableCache[T]) MultiSetWithTTL(ctx context.Context, valMap map[string]T, expiration time.Duration) error {
	for k, v := range valMap {
		if err := s.SetWithTTL(ctx, k, v, expiration); err != nil {
			return err
		}
	}
	return nil
}

func (s *spyExpirableCache[T]) Get(ctx context.Context, key string) (T, bool, error) {
	v, ok := s.store[key]
	return v, ok, nil
}

func (s *spyExpirableCache[T]) GetDel(ctx context.Context, key string) (T, bool, error) {
	v, ok := s.store[key]
	delete(s.store, key)
	return v, ok, nil
}

func (s *spyExpirableCache[T]) Del(ctx context.Context, key string) error {
	delete(s.store, key)
	return nil
}

func (s *spyExpirableCache[T]) Exists(ctx context.Context, key string) (bool, error) {
	_, ok := s.store[key]
	return ok, nil
}
