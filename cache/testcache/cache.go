package testcache

import (
	"context"
	"sync"
	"time"
)

type TestCache[S any] struct {
	rw         sync.RWMutex
	store      map[string]S
	expiration map[string]time.Time
}

func NewTestCache[S any]() *TestCache[S] {
	return &TestCache[S]{
		store:      make(map[string]S),
		expiration: make(map[string]time.Time),
	}
}

func (t *TestCache[S]) Set(ctx context.Context, key string, val S) error {
	t.rw.Lock()
	defer t.rw.Unlock()

	t.store[key] = val
	delete(t.expiration, key)
	return nil
}

func (t *TestCache[S]) SetWithTTL(ctx context.Context, key string, val S, expiration time.Duration) error {
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

func (t *TestCache[S]) MultiSet(ctx context.Context, valMap map[string]S) error {
	t.rw.Lock()
	defer t.rw.Unlock()

	for key, val := range valMap {
		t.store[key] = val
		delete(t.expiration, key)
	}
	return nil
}

func (t *TestCache[S]) MultiSetWithTTL(ctx context.Context, valMap map[string]S, expiration time.Duration) error {
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

func (t *TestCache[S]) Get(ctx context.Context, key string) (*S, error) {
	t.rw.RLock()
	defer t.rw.RUnlock()

	if exp, ok := t.expiration[key]; ok && time.Now().After(exp) {
		delete(t.store, key)
		delete(t.expiration, key)
		return nil, nil
	}

	val, ok := t.store[key]
	if !ok {
		return nil, nil
	}
	return &val, nil
}

func (t *TestCache[S]) MultiGet(ctx context.Context, keys []string) (map[string]S, error) {
	t.rw.RLock()
	defer t.rw.RUnlock()

	result := make(map[string]S)
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

func (t *TestCache[S]) Del(ctx context.Context, key string) error {
	t.rw.Lock()
	defer t.rw.Unlock()

	delete(t.store, key)
	delete(t.expiration, key)
	return nil
}

func (t *TestCache[S]) MultiDel(ctx context.Context, keys []string) error {
	t.rw.Lock()
	defer t.rw.Unlock()

	for _, key := range keys {
		delete(t.store, key)
		delete(t.expiration, key)
	}
	return nil
}

func (t *TestCache[S]) DelAll(ctx context.Context) error {
	t.rw.Lock()
	defer t.rw.Unlock()

	t.store = make(map[string]S)
	t.expiration = make(map[string]time.Time)
	return nil
}

func (t *TestCache[S]) Close() error {
	return nil
}
