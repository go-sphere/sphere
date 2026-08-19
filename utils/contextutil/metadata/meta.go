package metadata

import (
	"context"
	"maps"
)

type metaKey struct{}

var metaContextKey = metaKey{}

// WithMeta returns a new context with the provided metadata map attached.
// The metadata can be retrieved later using MetaFrom. A copy of the map is stored,
// so later mutations of the caller's original map do not affect the context value.
// Note that a nil or empty map still yields a non-nil (empty) map from MetaFrom.
func WithMeta(ctx context.Context, m map[string]any) context.Context {
	clone := make(map[string]any, len(m))
	maps.Copy(clone, m)
	return context.WithValue(ctx, metaContextKey, clone)
}

// MetaFrom extracts metadata from the given context.
// Returns nil if no metadata is found or if the stored value is not a valid metadata map.
//
// The returned map is the one held by the context, not a copy: it must be treated as
// read-only. Writing to it races with every other holder of the same context, and the
// copy made by WithMeta only protects against mutations of the caller's original map.
func MetaFrom(ctx context.Context) map[string]any {
	if v := ctx.Value(metaContextKey); v != nil {
		if m, ok := v.(map[string]any); ok {
			return m
		}
	}
	return nil
}
