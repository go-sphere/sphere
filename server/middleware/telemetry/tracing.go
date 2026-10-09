package telemetry

import (
	"net/http"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// scopeName is the instrumentation scope of spans and metrics.
const scopeName = "github.com/go-sphere/sphere/server/middleware/telemetry"

func newConfig(opts []Option) *config {
	c := &config{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// NewTracing returns middleware that traces each request with one server span.
// It extracts the remote parent from the request headers with the propagator
// (the global one by default, which is a no-op until the application calls
// otel.SetTextMapPropagator), names the span "METHOD route" using the matched
// pattern (just "METHOD" when no route matched, so the name stays
// low-cardinality), and publishes the span in ctx.Context(). Unless
// [WithoutLogAttrs] is set it also attaches trace_id and span_id to the log
// attrs of the context.
//
// Span status is Error only for responses of 500 and above, with the status
// text as description; 4xx stays Unset. The status is the one the client gets
// (see httpz.ErrorStatus). The middleware never recovers: on a panic or
// runtime.Goexit it marks the span as failed with error.type "panic", ends it,
// and lets the panic continue. Register it outermost, before [NewMetrics],
// requestid and logger middleware.
//
// Providers and the propagator are read at construction; the otel globals
// delegate, so installing an SDK later still takes effect. Nothing here sets a
// global.
func NewTracing(opts ...Option) httpx.Middleware {
	cfg := newConfig(opts)
	tp := cfg.tp
	if tp == nil {
		tp = otel.GetTracerProvider()
	}
	prop := cfg.propagator
	if prop == nil {
		prop = otel.GetTextMapPropagator()
	}
	tracer := tp.Tracer(scopeName)
	return func(next httpx.Handler) httpx.Handler {
		return func(ctx httpx.Context) error {
			if cfg.skip != nil && cfg.skip(ctx) {
				return next(ctx)
			}
			base := ctx.Context()
			remote := prop.Extract(base, headerCarrier{ctx})
			method, orig := normalizeMethod(ctx.Method())
			route := ctx.FullPath()

			attrs := make([]attribute.KeyValue, 0, 6)
			attrs = append(attrs, methodAttr(method), pathAttr(ctx.Path()))
			if orig != "" {
				attrs = append(attrs, methodOriginalAttr(orig))
			}
			if ua := ctx.Header("User-Agent"); ua != "" {
				attrs = append(attrs, userAgentAttr(ua))
			}
			if cfg.scheme != "" {
				attrs = append(attrs, schemeAttr(cfg.scheme))
			}
			name := method
			if route != "" {
				name += " " + route
				attrs = append(attrs, routeAttr(route))
			}
			startOpts := []trace.SpanStartOption{
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(attrs...),
			}
			parent := remote
			if cfg.public {
				startOpts = append(startOpts, trace.WithNewRoot())
				if link := trace.LinkFromContext(remote); link.SpanContext.IsValid() {
					startOpts = append(startOpts, trace.WithLinks(link))
				}
				parent = base
			}
			sc, span := tracer.Start(parent, name, startOpts...)
			if !cfg.noLogAttrs {
				sc = log.ContextWithAttrs(sc, TraceAttrs(sc)...)
			}
			ctx.SetContext(sc)

			finished := false
			defer func() {
				if !finished {
					span.SetStatus(codes.Error, "panic")
					span.SetAttributes(attribute.String(keyErrorType, "panic"))
					span.End()
				}
			}()
			err := next(ctx)
			finished = true

			// Routers may resolve the pattern only while dispatching.
			if route == "" {
				if route = ctx.FullPath(); route != "" {
					span.SetName(name + " " + route)
					span.SetAttributes(routeAttr(route))
				}
			}
			status := responseStatus(ctx, err)
			if status != 0 {
				span.SetAttributes(statusAttr(status))
			}
			if isServerError(status) && !clientCanceled(ctx, err) {
				if kv, ok := errorTypeAttr(status, false); ok {
					span.SetAttributes(kv)
				}
				span.SetStatus(codes.Error, http.StatusText(status))
				if err != nil && !cfg.noErrEvent {
					span.RecordError(err)
				}
			}
			span.End()
			return err
		}
	}
}
