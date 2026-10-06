package cache_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/mcache"
)

type user struct {
	Name string `json:"name"`
}

// Typed values over any ByteCache driver. The caller owns and closes the
// backend; closing the typed wrapper is a no-op.
func ExampleNewJsonCache() {
	ctx := context.Background()
	backend := mcache.NewByteCache()
	defer func() { _ = backend.Close() }()

	users := cache.NewJsonCache[user](backend)
	if err := users.SetWithTTL(ctx, "user:1", user{Name: "Ada"}, time.Minute); err != nil {
		fmt.Println("set:", err)
		return
	}
	u, found, err := users.Get(ctx, "user:1")
	if err != nil {
		fmt.Println("get:", err)
		return
	}
	fmt.Println(u.Name, found)

	_, found, err = users.Get(ctx, "user:2")
	fmt.Println(found, err)
	// Output:
	// Ada true
	// false <nil>
}

// Read-through loading: the builder runs on a miss and its result is stored,
// so the second call is served from the cache.
func ExampleGetEx() {
	ctx := context.Background()
	c := mcache.NewMapCache[user]()
	defer func() { _ = c.Close() }()

	calls := 0
	load := func() (user, error) {
		calls++
		return user{Name: "Grace"}, nil
	}
	for range 2 {
		u, found, err := cache.GetEx(ctx, c, "user:7", load, cache.WithExpiration(time.Minute))
		if err != nil {
			fmt.Println("load:", err)
			return
		}
		fmt.Println(u.Name, found)
	}
	fmt.Println("builder calls:", calls)
	// Output:
	// Grace true
	// Grace true
	// builder calls: 1
}

// A negative TTL is rejected by every driver and nothing is written.
func ExampleErrInvalidTTL() {
	ctx := context.Background()
	c := mcache.NewMapCache[string]()
	defer func() { _ = c.Close() }()

	err := c.SetWithTTL(ctx, "k", "v", -time.Second)
	fmt.Println(errors.Is(err, cache.ErrInvalidTTL))
	ok, err := c.Exists(ctx, "k")
	fmt.Println(ok, err)
	// Output:
	// true
	// false <nil>
}
