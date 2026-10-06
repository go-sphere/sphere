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
// Metadata from an earlier WithMeta on ctx is shadowed, not merged; to add a
// key, copy MetaFrom(ctx) into a new map first.
func WithMeta(ctx context.Context, m map[string]any) context.Context {
	clone := make(map[string]any, len(m))
	maps.Copy(clone, m)
	return context.WithValue(ctx, metaContextKey, clone)
}

// MetaFrom extracts metadata from the given context.
// It returns nil if ctx is nil or when WithMeta was never called. A stored value that is
// not a map also yields nil (not reachable through this package's API).
//
// The returned map is the one held by the context, not a copy: treat it as
// read-only. Writing to it races with every other holder of the same context.
func MetaFrom(ctx context.Context) map[string]any {
	if ctx == nil {
		return nil
	}
	if m, ok := ctx.Value(metaContextKey).(map[string]any); ok {
		return m
	}
	return nil
}
