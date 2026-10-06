package memory_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/memory"
)

func ExampleNewMemoryCache() {
	ctx := context.Background()
	c := memory.NewMemoryCache[string]()
	defer func() { _ = c.Close() }()

	// Writes are synchronous by default, so the value is visible right away.
	if err := c.Set(ctx, "greeting", "hello"); err != nil {
		fmt.Println("set:", err)
		return
	}
	v, found, err := c.Get(ctx, "greeting")
	fmt.Println(v, found, err)
	// Output: hello true <nil>
}

// After Close, every operation reports cache.ErrClosed.
func ExampleCache_Close() {
	c := memory.NewMemoryCache[string]()
	if err := c.Close(); err != nil {
		fmt.Println("close:", err)
		return
	}
	_, _, err := c.Get(context.Background(), "k")
	fmt.Println(errors.Is(err, cache.ErrClosed))
	// Output: true
}
