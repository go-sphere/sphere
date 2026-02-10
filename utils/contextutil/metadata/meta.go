// Package metadata provides utilities for attaching and retrieving metadata from Go contexts.
// It offers both simple key-value storage and a specialized context implementation
// that supports string-keyed metadata for efficient data passing between function calls.
package metadata

import "context"

type metaKey struct{}

var metaContextKey = metaKey{}

// WithMeta returns a new context with the provided metadata map attached.
// The metadata can be retrieved later using MetaFrom.
func WithMeta(ctx context.Context, m map[string]any) context.Context {
	return context.WithValue(ctx, metaContextKey, m)
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
