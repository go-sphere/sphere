package metadata

import (
	"context"
	"maps"
)

type metaKey struct{}

var metaContextKey = metaKey{}

// WithMeta returns a new context with the provided metadata map attached.
// The metadata can be retrieved later using MetaFrom. A copy of the map is stored
// so that later mutations of the caller's original map do not affect the context
// value, avoiding data races on the shared reference.
func WithMeta(ctx context.Context, m map[string]any) context.Context {
	clone := make(map[string]any, len(m))
	maps.Copy(clone, m)
	return context.WithValue(ctx, metaContextKey, clone)
}

// MetaFrom extracts metadata from the given context.
// Returns nil if no metadata is found or if the stored value is not a valid metadata map.
func MetaFrom(ctx context.Context) map[string]any {
	if v := ctx.Value(metaContextKey); v != nil {
		if m, ok := v.(map[string]any); ok {
			return m
		}
	}
	return nil
}
