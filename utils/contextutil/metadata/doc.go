// Package metadata attaches a map[string]any to a context.Context, for
// request-scoped values (trace tags, client info) that travel with ctx.
//
// # Usage
//
//	import "github.com/go-sphere/sphere/utils/contextutil/metadata"
//
//	ctx = metadata.WithMeta(ctx, map[string]any{"client": "ios"})
//	if meta := metadata.MetaFrom(ctx); meta != nil {
//		client, _ := meta["client"].(string)
//	}
//
// # Rules
//
//   - WithMeta stores a copy of the map; later changes to the caller's map
//     are not visible. Each call replaces, not merges, earlier metadata.
//   - A nil or empty map still yields a non-nil empty map from MetaFrom.
//     MetaFrom returns nil only when ctx is nil or WithMeta was never called.
//   - The map returned by MetaFrom is the one held by the context: treat it as
//     read-only, since writing to it races with other holders of ctx.
package metadata
