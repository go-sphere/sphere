package mcache

import (
	"context"
	"sync"
	"time"
)

// Map is a simple in-memory cache implementation using Go's built-in map with read-write mutex protection.
// It supports TTL-based expiration and is suitable for lightweight caching needs without external dependencies.
type Map[K comparable, S any] struct {
	rw         sync.RWMutex
	store      map[K]S
	expiration map[K]time.Time
}

// NewMapCache creates a new map-based cache for string keys and typed values.
// This is a lightweight alternative to more complex caching solutions.
func NewMapCache[S any]() *Map[string, S] {
	return &Map[string, S]{
		store:      make(map[string]S),
		expiration: make(map[string]time.Time),
	}
}

func (t *Map[K, S]) Set(ctx context.Context, key K, val S) error {
	t.rw.Lock()
	defer t.rw.Unlock()

	t.store[key] = val
	delete(t.expiration, key)
	return nil
}

func (t *Map[K, S]) SetWithTTL(ctx context.Context, key K, val S, expiration time.Duration) error {
	t.rw.Lock()
	defer t.rw.Unlock()

	t.store[key] = val
	if expiration >= 0 {
		t.expiration[key] = time.Now().Add(expiration)
	} else {
		delete(t.expiration, key)
	}
	return nil
}

func (t *Map[K, S]) MultiSet(ctx context.Context, valMap map[K]S) error {
	t.rw.Lock()
	defer t.rw.Unlock()

	for key, val := range valMap {
		t.store[key] = val
		delete(t.expiration, key)
	}
	return nil
}

func (t *Map[K, S]) MultiSetWithTTL(ctx context.Context, valMap map[K]S, expiration time.Duration) error {
	t.rw.Lock()
	defer t.rw.Unlock()

	now := time.Now()
	for key, val := range valMap {
		t.store[key] = val
		if expiration >= 0 {
			t.expiration[key] = now.Add(expiration)
		} else {
			delete(t.expiration, key)
		}
	}
	return nil
}

func (t *Map[K, S]) Get(ctx context.Context, key K) (S, bool, error) {
	t.rw.RLock()
	defer t.rw.RUnlock()

	if exp, ok := t.expiration[key]; ok && time.Now().After(exp) {
		delete(t.store, key)
		delete(t.expiration, key)
		var zeroValue S
		return zeroValue, false, nil
	}
	val, ok := t.store[key]
	return val, ok, nil
}

func (t *Map[K, S]) MultiGet(ctx context.Context, keys []K) (map[K]S, error) {
	t.rw.RLock()
	defer t.rw.RUnlock()

	result := make(map[K]S, len(keys))
	now := time.Now()

	for _, key := range keys {
		if exp, ok := t.expiration[key]; ok && now.After(exp) {
			delete(t.store, key)
			delete(t.expiration, key)
			continue
		}

		if val, ok := t.store[key]; ok {
			result[key] = val
		}
	}

	return result, nil
}

func (t *Map[K, S]) Del(ctx context.Context, key K) error {
	t.rw.Lock()
	defer t.rw.Unlock()

	delete(t.store, key)
	delete(t.expiration, key)
	return nil
}

func (t *Map[K, S]) MultiDel(ctx context.Context, keys []K) error {
	t.rw.Lock()
	defer t.rw.Unlock()

	for _, key := range keys {
		delete(t.store, key)
		delete(t.expiration, key)
	}
	return nil
}

func (t *Map[K, S]) DelAll(ctx context.Context) error {
	t.rw.Lock()
	defer t.rw.Unlock()

	t.store = make(map[K]S)
	t.expiration = make(map[K]time.Time)
	return nil
}

func (t *Map[K, S]) Count() int {
	t.rw.Lock()
	defer t.rw.Unlock()

	now := time.Now()
	for key, exp := range t.expiration {
		if now.After(exp) {
			delete(t.store, key)
			delete(t.expiration, key)
		}
	}
	return len(t.store)
}

func (t *Map[K, S]) Trim() {
	_ = t.Count()
}

func (t *Map[K, S]) Exists(ctx context.Context, key K) (bool, error) {
	_, ok, err := t.Get(ctx, key)
	return ok, err
}

func (t *Map[K, S]) Close() error {
	return nil
}
