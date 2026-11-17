package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-sphere/confstore/codec"
)

type multiSetSpy struct {
	ByteCache
	multiSetCalls        int
	multiSetWithTTLCalls int
}

func (s *multiSetSpy) MultiSet(context.Context, map[string][]byte) error {
	s.multiSetCalls++
	return nil
}

func (s *multiSetSpy) MultiSetWithTTL(context.Context, map[string][]byte, time.Duration) error {
	s.multiSetWithTTLCalls++
	return nil
}

func TestCodecCacheBatchMarshalFailureDoesNotCallBackend(t *testing.T) {
	marshalErr := errors.New("marshal failed")
	failingCodec := codec.NewCodec(
		func(any) ([]byte, error) { return nil, marshalErr },
		func([]byte, any) error { return nil },
	)
	backend := &multiSetSpy{}
	cache := NewCodecCache[int](backend, failingCodec)

	if err := cache.MultiSet(t.Context(), map[string]int{"k": 1}); !errors.Is(err, marshalErr) {
		t.Fatalf("MultiSet error = %v, want %v", err, marshalErr)
	}
	if err := cache.MultiSetWithTTL(t.Context(), map[string]int{"k": 1}, time.Minute); !errors.Is(err, marshalErr) {
		t.Fatalf("MultiSetWithTTL error = %v, want %v", err, marshalErr)
	}
	if backend.multiSetCalls != 0 || backend.multiSetWithTTLCalls != 0 {
		t.Fatalf("backend calls = (%d, %d), want (0, 0)", backend.multiSetCalls, backend.multiSetWithTTLCalls)
	}
}

type mockByteCache struct {
	data map[string][]byte
}

func (m *mockByteCache) Set(context.Context, string, []byte) error                       { return nil }
func (m *mockByteCache) SetWithTTL(context.Context, string, []byte, time.Duration) error { return nil }
func (m *mockByteCache) MultiSet(context.Context, map[string][]byte) error               { return nil }
func (m *mockByteCache) MultiSetWithTTL(context.Context, map[string][]byte, time.Duration) error {
	return nil
}
func (m *mockByteCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	v, ok := m.data[key]
	return v, ok, nil
}
func (m *mockByteCache) GetDel(context.Context, string) ([]byte, bool, error) { return nil, false, nil }
func (m *mockByteCache) MultiGet(_ context.Context, keys []string) (map[string][]byte, error) {
	out := make(map[string][]byte, len(keys))
	for _, k := range keys {
		if v, ok := m.data[k]; ok {
			out[k] = v
		}
	}
	return out, nil
}
func (m *mockByteCache) Del(context.Context, string) error        { return nil }
func (m *mockByteCache) MultiDel(context.Context, []string) error { return nil }
func (m *mockByteCache) DelAll(context.Context) error             { return nil }
func (m *mockByteCache) Exists(context.Context, string) (bool, error) {
	return false, nil
}
func (m *mockByteCache) Close() error { return nil }

func TestCodecCacheNegativeTTLSkipsMarshalAndMultiGetDeduplicatesKeys(t *testing.T) {
	marshalCalls := 0
	unmarshalCalls := 0
	spyCodec := codec.NewCodec(
		func(v any) ([]byte, error) {
			marshalCalls++
			return []byte("1"), nil
		},
		func(_ []byte, v any) error {
			unmarshalCalls++
			*(v.(*int)) = 7
			return nil
		},
	)
	backend := &mockByteCache{data: map[string][]byte{"k": []byte("1")}}
	cc := NewCodecCache[int](backend, spyCodec)

	if err := cc.SetWithTTL(t.Context(), "k", 1, -time.Second); !errors.Is(err, ErrInvalidTTL) {
		t.Fatalf("SetWithTTL(-1s) = %v, want ErrInvalidTTL", err)
	}
	if err := cc.MultiSetWithTTL(t.Context(), map[string]int{"k": 1}, -time.Second); !errors.Is(err, ErrInvalidTTL) {
		t.Fatalf("MultiSetWithTTL(-1s) = %v, want ErrInvalidTTL", err)
	}
	if marshalCalls != 0 {
		t.Fatalf("marshalCalls = %d, want 0 when expiration < 0", marshalCalls)
	}

	got, err := cc.MultiGet(t.Context(), []string{"k", "k", "k"})
	if err != nil || got["k"] != 7 {
		t.Fatalf("MultiGet = (%v, %v), want k=7", got, err)
	}
	if unmarshalCalls != 1 {
		t.Fatalf("unmarshalCalls = %d, want 1 for duplicate keys", unmarshalCalls)
	}
}
