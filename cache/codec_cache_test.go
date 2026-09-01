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
