package nscache_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-sphere/sphere/cache/mcache"
	"github.com/go-sphere/sphere/cache/nscache"
)

// Two namespaces share one backend; DelAll clears only its own namespace.
func ExampleNewNSCacheChecked() {
	ctx := context.Background()
	backend := mcache.NewByteCache()
	defer func() { _ = backend.Close() }()

	sessions, err := nscache.NewNSCacheChecked[[]byte]("session", backend)
	if err != nil {
		fmt.Println(err)
		return
	}
	tokens, err := nscache.NewNSCacheChecked[[]byte]("token", backend)
	if err != nil {
		fmt.Println(err)
		return
	}
	if err := sessions.Set(ctx, "abc", []byte("s")); err != nil {
		fmt.Println(err)
		return
	}
	if err := tokens.Set(ctx, "abc", []byte("t")); err != nil {
		fmt.Println(err)
		return
	}

	raw, found, err := backend.Get(ctx, "session:abc")
	fmt.Println(string(raw), found, err)

	if err := sessions.DelAll(ctx); err != nil {
		fmt.Println(err)
		return
	}
	_, found, err = sessions.Get(ctx, "abc")
	fmt.Println("session found:", found, err)
	_, found, err = tokens.Get(ctx, "abc")
	fmt.Println("token found:", found, err)
	// Output:
	// s true <nil>
	// session found: false <nil>
	// token found: true <nil>
}

func ExampleNewNSCacheChecked_invalidNamespace() {
	_, err := nscache.NewNSCacheChecked[[]byte]("a:b", mcache.NewByteCache())
	fmt.Println(errors.Is(err, nscache.ErrInvalidNamespace))
	// Output: true
}
