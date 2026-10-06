package mcache_test

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/go-sphere/sphere/cache/mcache"
)

func ExampleNewMapCache() {
	ctx := context.Background()
	c := mcache.NewMapCache[int]()
	defer func() { _ = c.Close() }()

	if err := c.SetWithTTL(ctx, "visits", 1, time.Minute); err != nil {
		fmt.Println("set:", err)
		return
	}
	v, found, err := c.Get(ctx, "visits")
	fmt.Println(v, found, err)

	// Drop expired entries; call periodically in long-running processes.
	c.Trim()
	fmt.Println(c.Count())
	// Output:
	// 1 true <nil>
	// 1
}

func ExampleMap_Keys() {
	ctx := context.Background()
	c := mcache.NewMapCache[string]()
	if err := c.MultiSet(ctx, map[string]string{"user:1": "a", "user:2": "b", "order:1": "c"}); err != nil {
		fmt.Println("set:", err)
		return
	}
	keys, err := c.Keys(ctx, "user:")
	if err != nil {
		fmt.Println("keys:", err)
		return
	}
	slices.Sort(keys) // order is unspecified
	fmt.Println(keys)
	// Output: [user:1 user:2]
}
