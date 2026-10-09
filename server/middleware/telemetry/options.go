package telemetry

import (
	"github.com/go-sphere/httpx"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// UnmatchedRoute is the http.route metric value for requests that matched no
// route; it equals logger.UnmatchedRoute. Spans omit http.route instead.
const UnmatchedRoute = "unmatched"

type config struct {
	tp         trace.TracerProvider
	mp         metric.MeterProvider
	propagator propagation.TextMapPropagator
	skip       func(httpx.Context) bool
	public     bool
	noLogAttrs bool
	noErrEvent bool
	scheme     string

	durationSet bool
	duration    metric.Float64Histogram
	activeSet   bool
	active      metric.Int64UpDownCounter
}

// Option configures [NewTracing] and [NewMetrics]. Each option documents the
// constructor it affects; the other constructor ignores it.
type Option func(*config)

// WithTracerProvider sets the provider [NewTracing] starts spans from. nil
// keeps the global provider.
func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(c *config) {
		if tp != nil {
			c.tp = tp
		}
	}
}

// WithMeterProvider sets the provider [NewMetrics] creates instruments from.
// nil keeps the global provider.
func WithMeterProvider(mp metric.MeterProvider) Option {
	return func(c *config) {
		if mp != nil {
			c.mp = mp
		}
	}
}

// WithPropagators sets the propagator [NewTracing] extracts the remote parent
// with. nil keeps the global propagator.
func WithPropagators(p propagation.TextMapPropagator) Option {
	return func(c *config) {
		if p != nil {
			c.propagator = p
		}
	}
}

// WithSkip makes both constructors pass requests straight through when fn
// returns true. fn runs before the chain and may rely only on ctx.Path and
// ctx.Method. A nil fn skips nothing.
func WithSkip(fn func(httpx.Context) bool) Option {
	return func(c *config) { c.skip = fn }
}

// WithPublicEndpoint makes [NewTracing] ignore the remote parent: it starts a
// new root span and links to the remote span context. Use it on edges that face
// untrusted callers, who could otherwise force sampling or pick trace IDs.
func WithPublicEndpoint() Option { return func(c *config) { c.public = true } }

// WithoutLogAttrs stops [NewTracing] from attaching trace_id and span_id to the
// request context with log.ContextWithAttrs. Use it together with
// log.WrapBackendWithContextMerge and [TraceAttrs], not alongside.
func WithoutLogAttrs() Option { return func(c *config) { c.noLogAttrs = true } }

// WithoutErrorEvents stops [NewTracing] from recording the error of a 5xx
// response as a span event; its text may carry internal detail such as SQL.
func WithoutErrorEvents() Option { return func(c *config) { c.noErrEvent = true } }

// WithURLScheme sets the url.scheme attribute on spans and metrics. It is
// omitted by default because httpx.Context does not expose the scheme.
func WithURLScheme(scheme string) Option { return func(c *config) { c.scheme = scheme } }

// WithDurationHistogram makes [NewMetrics] record request duration, in
// seconds, to h instead of the default http.server.request.duration
// instrument. nil disables the instrument.
func WithDurationHistogram(h metric.Float64Histogram) Option {
	return func(c *config) { c.durationSet, c.duration = true, h }
}

// WithActiveRequests makes [NewMetrics] track in-flight requests with counter
// instead of the default http.server.active_requests instrument. nil disables
// the instrument.
func WithActiveRequests(counter metric.Int64UpDownCounter) Option {
	return func(c *config) { c.activeSet, c.active = true, counter }
}

// WithoutActiveRequests disables the in-flight request instrument. That
// semconv metric is still in Development status.
func WithoutActiveRequests() Option { return WithActiveRequests(nil) }
