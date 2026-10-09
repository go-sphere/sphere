package log

import (
	"context"
	"slices"
)

type ctxAttrsKey struct{}

// ContextWithAttrs returns a copy of ctx that carries attrs in addition to any
// already attached with ContextWithAttrs. Loggers built by this package
// ([NewLogger] and the global logger) append them to every entry written
// through a Context method (InfoContext and the like) with that ctx; the
// context-free methods never see them. ctx must be non-nil; with no attrs it is
// returned unchanged. The attached slice is copied, so callers may reuse attrs.
func ContextWithAttrs(ctx context.Context, attrs ...Attr) context.Context {
	if len(attrs) == 0 {
		return ctx
	}
	prev, _ := ctx.Value(ctxAttrsKey{}).([]Attr)
	merged := make([]Attr, 0, len(prev)+len(attrs))
	merged = append(merged, prev...)
	merged = append(merged, attrs...)
	return context.WithValue(ctx, ctxAttrsKey{}, merged)
}

// AttrsFromContext returns the attrs attached to ctx by ContextWithAttrs, oldest
// first, or nil when there are none. The returned slice is a copy. A nil ctx
// yields nil.
func AttrsFromContext(ctx context.Context) []Attr {
	if ctx == nil {
		return nil
	}
	attrs, _ := ctx.Value(ctxAttrsKey{}).([]Attr)
	return slices.Clone(attrs)
}

// withContextAttrs appends the ctx-carried attrs after the call-site attrs.
func withContextAttrs(ctx context.Context, attrs []Attr) []Attr {
	if ctx == nil {
		return attrs
	}
	extra, _ := ctx.Value(ctxAttrsKey{}).([]Attr)
	if len(extra) == 0 {
		return attrs
	}
	return append(slices.Clone(attrs), extra...)
}
