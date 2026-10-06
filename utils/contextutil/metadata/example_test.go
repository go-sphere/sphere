package metadata_test

import (
	"context"
	"fmt"
	"maps"

	"github.com/go-sphere/sphere/utils/contextutil/metadata"
)

func ExampleWithMeta() {
	ctx := context.Background()
	fmt.Println(metadata.MetaFrom(ctx) == nil)

	ctx = metadata.WithMeta(ctx, map[string]any{"client": "ios"})
	client, _ := metadata.MetaFrom(ctx)["client"].(string)
	fmt.Println(client)

	// WithMeta replaces earlier metadata; copy it to extend instead.
	extended := maps.Clone(metadata.MetaFrom(ctx))
	extended["version"] = "1.2.0"
	ctx = metadata.WithMeta(ctx, extended)
	fmt.Println(len(metadata.MetaFrom(ctx)))
	// Output:
	// true
	// ios
	// 2
}
