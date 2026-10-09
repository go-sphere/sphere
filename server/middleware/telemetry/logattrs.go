package telemetry

import (
	"context"

	"github.com/go-sphere/sphere/log"
	"go.opentelemetry.io/otel/trace"
)

// Log attr keys written for the active span.
const (
	TraceIDKey = "trace_id"
	SpanIDKey  = "span_id"
)

// TraceAttrs returns the trace_id and span_id attrs of the span in ctx, or nil
// when ctx carries no valid span context. It satisfies log.ContextAttrExtractor
// for use with log.WrapBackendWithContextMerge.
func TraceAttrs(ctx context.Context) []log.Attr {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return nil
	}
	return []log.Attr{
		log.String(TraceIDKey, sc.TraceID().String()),
		log.String(SpanIDKey, sc.SpanID().String()),
	}
}

// TraceID returns the hex trace ID of the span in ctx, or "" without one.
func TraceID(ctx context.Context) string {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		return sc.TraceID().String()
	}
	return ""
}

// SpanID returns the hex span ID of the span in ctx, or "" without one.
func SpanID(ctx context.Context) string {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		return sc.SpanID().String()
	}
	return ""
}
