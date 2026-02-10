package log

import (
	"context"
	"sort"
)

// ContextAttrExtractor extracts attributes from context for context-aware logging.
type ContextAttrExtractor func(ctx context.Context) []Attr

// ContextMapExtractor extracts key-value pairs from context for logging.
type ContextMapExtractor func(ctx context.Context) map[string]any

// MapContextAttrExtractor adapts map extractors into attr extractors.
func MapContextAttrExtractor(extractor ContextMapExtractor) ContextAttrExtractor {
	if extractor == nil {
		return nil
	}
	return func(ctx context.Context) []Attr {
		m := extractor(ctx)
		if len(m) == 0 {
			return nil
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		attrs := make([]Attr, 0, len(m))
		for _, k := range keys {
			attrs = append(attrs, Any(k, m[k]))
		}
		return attrs
	}
}

// MergeAttrs merges base attrs with explicit attrs. Explicit attrs override same-key values from base.
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

// WrapBackendWithContextMerge returns a backend that merges attrs extracted from context.
func WrapBackendWithContextMerge(backend Backend, extractor ContextAttrExtractor) Backend {
	if backend == nil || extractor == nil {
		return backend
	}
	return &contextMergeBackend{next: backend, extractor: extractor}
}

// WrapBackendWithContextMapMerge returns a backend wrapper using a map-based context extractor.
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
