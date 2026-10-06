package log

import (
	"context"
	"maps"
	"slices"
)

// ContextAttrExtractor extracts attributes from context for context-aware logging.
type ContextAttrExtractor func(ctx context.Context) []Attr

// ContextMapExtractor extracts key-value pairs from context for logging.
type ContextMapExtractor func(ctx context.Context) map[string]any

// MapContextAttrExtractor adapts a map extractor into an attr extractor that
// emits Any attrs in sorted key order. A nil extractor yields nil.
func MapContextAttrExtractor(extractor ContextMapExtractor) ContextAttrExtractor {
	if extractor == nil {
		return nil
	}
	return func(ctx context.Context) []Attr {
		m := extractor(ctx)
		if len(m) == 0 {
			return nil
		}
		keys := slices.Sorted(maps.Keys(m))
		attrs := make([]Attr, 0, len(m))
		for _, k := range keys {
			attrs = append(attrs, Any(k, m[k]))
		}
		return attrs
	}
}

// MergeAttrs merges base attrs with explicit attrs. Explicit attrs override
// same-key values from base in place, keeping the first position of each key.
// When either slice is empty the other is returned unchanged (not copied).
func MergeAttrs(base []Attr, explicit []Attr) []Attr {
	if len(base) == 0 {
		return explicit
	}
	if len(explicit) == 0 {
		return base
	}
	out := make([]Attr, 0, len(base)+len(explicit))
	index := make(map[string]int, len(base)+len(explicit))
	for _, a := range base {
		// A repeated key in base overwrites its earlier slot instead of
		// appending: two attrs under one key are what the merge exists to
		// avoid, and leaving both would let a later explicit override patch
		// only the last one.
		if i, ok := index[a.Key]; ok {
			out[i] = a
			continue
		}
		index[a.Key] = len(out)
		out = append(out, a)
	}
	for _, a := range explicit {
		if i, ok := index[a.Key]; ok {
			out[i] = a
			continue
		}
		index[a.Key] = len(out)
		out = append(out, a)
	}
	return out
}

// WrapBackendWithContextMerge returns a backend that calls extractor on the
// Log context and merges the result before the explicit attrs, which win on
// key collisions (see MergeAttrs). If backend or extractor is nil it returns
// backend unchanged. The wrapper keeps the extractor across With, forwards
// Sync, and implements Close by forwarding to the wrapped backend when it has
// one. Only the Context logging methods carry a caller context; the others
// pass context.Background to the extractor.
func WrapBackendWithContextMerge(backend Backend, extractor ContextAttrExtractor) Backend {
	if backend == nil || extractor == nil {
		return backend
	}
	return &contextMergeBackend{next: backend, extractor: extractor}
}

// WrapBackendWithContextMapMerge is WrapBackendWithContextMerge with a
// map-based extractor; map entries become attrs in sorted key order.
func WrapBackendWithContextMapMerge(backend Backend, extractor ContextMapExtractor) Backend {
	return WrapBackendWithContextMerge(backend, MapContextAttrExtractor(extractor))
}

type contextMergeBackend struct {
	next      Backend
	extractor ContextAttrExtractor
}

func (b *contextMergeBackend) Log(ctx context.Context, level Level, msg string, attrs ...Attr) {
	b.next.Log(ctx, level, msg, MergeAttrs(b.extractor(ctx), attrs)...)
}

func (b *contextMergeBackend) Sync() error {
	return b.next.Sync()
}

func (b *contextMergeBackend) With(options ...Option) Backend {
	return &contextMergeBackend{
		next:      b.next.With(options...),
		extractor: b.extractor,
	}
}

// Close forwards to the wrapped backend when it owns a handle, mirroring
// MultiBackend.Close. Without this the documented release pattern
// — type-asserting the installed backend to io.Closer — silently skips a
// wrapped backend, and the file handle it holds is never released. Wrapping is
// the recommended way to inject context fields, so this is the common shape.
func (b *contextMergeBackend) Close() error {
	if closer, ok := b.next.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}
