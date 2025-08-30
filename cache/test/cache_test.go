package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-sphere/sphere/cache/mcache"
	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/core/safe"
)

// This file only contains benchmark tests for performance comparison

func BenchmarkMemoryCacheSet(b *testing.B) {
	cache := memory.NewByteCache()
	defer safe.IfErrorPresent(cache.Close)
	ctx := context.Background()
	value := []byte("benchmark_value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i)
		err := cache.Set(ctx, key, value)
		if err != nil {
			b.Fatalf("Set failed: %v", err)
		}
	}
}

func BenchmarkMemoryCacheGet(b *testing.B) {
	cache := memory.NewByteCache()
	defer safe.IfErrorPresent(cache.Close)
	ctx := context.Background()
	value := []byte("benchmark_value")

	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key_%d", i)
		_ = cache.Set(ctx, key, value)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i%1000)
		_, _, err := cache.Get(ctx, key)
		if err != nil {
			b.Fatalf("Get failed: %v", err)
		}
	}
}

func BenchmarkMCacheSet(b *testing.B) {
	cache := mcache.NewMapCache[[]byte]()
	defer safe.IfErrorPresent(cache.Close)
	ctx := context.Background()
	value := []byte("benchmark_value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i)
		err := cache.Set(ctx, key, value)
		if err != nil {
			b.Fatalf("Set failed: %v", err)
		}
	}
}

func BenchmarkMCacheGet(b *testing.B) {
	cache := mcache.NewMapCache[[]byte]()
	defer safe.IfErrorPresent(cache.Close)
	ctx := context.Background()
	value := []byte("benchmark_value")

	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key_%d", i)
		_ = cache.Set(ctx, key, value)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i%1000)
		_, _, err := cache.Get(ctx, key)
		if err != nil {
			b.Fatalf("Get failed: %v", err)
		}
	}
}
