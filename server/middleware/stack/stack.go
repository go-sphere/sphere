package stack

import (
	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/server/middleware/logger"
	"github.com/go-sphere/sphere/server/middleware/requestid"
	"github.com/go-sphere/sphere/server/middleware/telemetry"
)

// Layer names, in stack order, as reported by the internal planner.
const (
	layerTracing   = "tracing"
	layerMetrics   = "metrics"
	layerRequestID = "requestid"
	layerAccessLog = "accesslog"
	layerRecovery  = "recovery"
)

type config struct {
	lg log.BaseLogger

	tracing     bool
	tracingOpts []telemetry.Option
	metrics     bool
	metricsOpts []telemetry.Option

	noRequestID   bool
	requestIDOpts []requestid.Option

	noAccessLog bool

	noRecovery    bool
	noStack       bool
	recoveryError bool
}

// Option configures [New] and [Apply].
type Option func(*config)

// WithLogger sets the logger for the access log and for panic reports. nil
// keeps the default: no access log, and panics logged to a stdio logger.
func WithLogger(lg log.BaseLogger) Option {
	return func(c *config) { c.lg = lg }
}

// WithTracing enables telemetry.NewTracing with opts. It is off by default.
func WithTracing(opts ...telemetry.Option) Option {
	return func(c *config) { c.tracing, c.tracingOpts = true, opts }
}

// WithMetrics enables telemetry.NewMetrics with opts. It is off by default.
// [New] returns the instrument-creation error of NewMetrics.
func WithMetrics(opts ...telemetry.Option) Option {
	return func(c *config) { c.metrics, c.metricsOpts = true, opts }
}

// WithRequestID configures the request-ID layer, which is on by default.
func WithRequestID(opts ...requestid.Option) Option {
	return func(c *config) { c.noRequestID, c.requestIDOpts = false, opts }
}

// WithoutRequestID removes the request-ID layer.
func WithoutRequestID() Option {
	return func(c *config) { c.noRequestID = true }
}

// WithoutAccessLog removes the access-log layer even when a logger is set.
func WithoutAccessLog() Option {
	return func(c *config) { c.noAccessLog = true }
}

// WithoutRecovery removes the panic-recovery layer. A panic then propagates to
// the adapter; the telemetry layers still end their span and record it.
func WithoutRecovery() Option {
	return func(c *config) { c.noRecovery = true }
}

// WithoutRecoveryStack stops panic reports from carrying a stack trace, which
// they do by default.
func WithoutRecoveryStack() Option {
	return func(c *config) { c.noStack = true }
}

// WithRecoveryError uses logger.RecoveryLogErr instead of logger.RecoveryLog,
// so a recovered panic surfaces as a *logger.PanicError to the layers outside
// it: the span records it as an event and the access entry is logged at Error.
func WithRecoveryError() Option {
	return func(c *config) { c.recoveryError = true }
}

type layer struct {
	name string
	mw   httpx.Middleware
}

// plan resolves opts into the ordered layers. This function is the single
// place that fixes the order documented on the package.
func plan(opts []Option) ([]layer, error) {
	var c config
	for _, opt := range opts {
		opt(&c)
	}
	var layers []layer
	if c.tracing {
		layers = append(layers, layer{layerTracing, telemetry.NewTracing(c.tracingOpts...)})
	}
	if c.metrics {
		m, err := telemetry.NewMetrics(c.metricsOpts...)
		if err != nil {
			return nil, err
		}
		layers = append(layers, layer{layerMetrics, m})
	}
	if !c.noRequestID {
		layers = append(layers, layer{layerRequestID, requestid.New(c.requestIDOpts...)})
	}
	if c.lg != nil && !c.noAccessLog {
		layers = append(layers, layer{layerAccessLog, logger.Log(c.lg)})
	}
	if !c.noRecovery {
		lg := c.lg
		if lg == nil {
			lg = log.NewLogger(log.NewStdioBackend())
		}
		rec := logger.RecoveryLog
		if c.recoveryError {
			rec = logger.RecoveryLogErr
		}
		layers = append(layers, layer{layerRecovery, rec(lg, !c.noStack)})
	}
	return layers, nil
}

// New returns the enabled layers outermost first, in the order documented on
// the package. The slice is freshly allocated and may be passed to
// engine.Use(mws...) or extended. The error comes from creating the metrics
// instruments and is nil unless [WithMetrics] is used.
func New(opts ...Option) ([]httpx.Middleware, error) {
	layers, err := plan(opts)
	if err != nil {
		return nil, err
	}
	mws := make([]httpx.Middleware, len(layers))
	for i, l := range layers {
		mws[i] = l.mw
	}
	return mws, nil
}

// Apply registers the stack from [New] on s, normally the engine, so that it
// also covers requests no route matched. Call it before registering routes
// and before adding other engine-level middleware such as CORS, which belongs
// inside the stack. It returns the error of [New] and registers nothing then.
func Apply(s httpx.MiddlewareScope, opts ...Option) error {
	mws, err := New(opts...)
	if err != nil {
		return err
	}
	s.Use(mws...)
	return nil
}
