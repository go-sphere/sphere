package nocache_test

import (
	"context"
	"fmt"

	"github.com/go-sphere/sphere/cache/nocache"
)

func ExampleNewNoCache() {
	ctx := context.Background()
	c := nocache.NewNoCache[string]()

	err := c.Set(ctx, "k", "v")
	fmt.Println(err)
	_, found, err := c.Get(ctx, "k")
	fmt.Println(found, err)
	// Output:
	// <nil>
	// false <nil>
}
