package telemetry

import (
	"time"

	"github.com/go-sphere/httpx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/semconv/v1.43.0/httpconv"
)

// NewMetrics returns middleware that records the HTTP server metrics:
// http.server.request.duration (seconds) and http.server.active_requests. The
// duration histogram's count is the request count; there is no separate
// counter.
//
// Duration attributes are http.request.method (non-standard methods become
// "_OTHER"), http.route (the matched pattern, or [UnmatchedRoute]),
// http.response.status_code, error.type for 5xx or "panic", and url.scheme when
// [WithURLScheme] is set. The status is the one [NewTracing] records. On a
// panic the duration is recorded with error.type "panic" and no status, the
// in-flight count is restored, and the panic continues.
//
// Instruments come from the meter provider (the global one by default) unless
// replaced with [WithDurationHistogram] or [WithActiveRequests]; passing nil
// disables one. With both disabled the middleware passes straight through.
// Register it directly inside [NewTracing] so recordings carry the span
// context. The error is from instrument creation.
func NewMetrics(opts ...Option) (httpx.Middleware, error) {
	cfg := newConfig(opts)
	var meter metric.Meter
	getMeter := func() metric.Meter {
		if meter == nil {
			mp := cfg.mp
			if mp == nil {
				mp = otel.GetMeterProvider()
			}
			meter = mp.Meter(scopeName)
		}
		return meter
	}
	dur, active := cfg.duration, cfg.active
	if !cfg.durationSet {
		d, err := httpconv.NewServerRequestDuration(getMeter())
		if err != nil {
			return nil, err
		}
		dur = d.Inst()
	}
	if !cfg.activeSet {
		a, err := httpconv.NewServerActiveRequests(getMeter())
		if err != nil {
			return nil, err
		}
		active = a.Inst()
	}
	if dur == nil && active == nil {
		return func(next httpx.Handler) httpx.Handler { return next }, nil
	}
	return func(next httpx.Handler) httpx.Handler {
		return func(ctx httpx.Context) error {
			if cfg.skip != nil && cfg.skip(ctx) {
				return next(ctx)
			}
			start := time.Now()
			method, _ := normalizeMethod(ctx.Method())
			rc := ctx.Context()

			var activeOpt metric.AddOption
			if active != nil && active.Enabled(rc) {
				kvs := make([]attribute.KeyValue, 0, 2)
				kvs = append(kvs, methodAttr(method))
				if cfg.scheme != "" {
					kvs = append(kvs, schemeAttr(cfg.scheme))
				}
				activeOpt = metric.WithAttributeSet(attribute.NewSet(kvs...))
				active.Add(rc, 1, activeOpt)
			}

			finished := false
			var err error
			record := func(panicked bool) {
				if activeOpt != nil {
					active.Add(rc, -1, activeOpt)
				}
				if dur == nil || !dur.Enabled(rc) {
					return
				}
				route := ctx.FullPath()
				if route == "" {
					route = UnmatchedRoute
				}
				kvs := make([]attribute.KeyValue, 0, 5)
				kvs = append(kvs, methodAttr(method), routeAttr(route))
				status := 0
				if !panicked {
					status = responseStatus(ctx, err)
					if status != 0 {
						kvs = append(kvs, statusAttr(status))
					}
				}
				if kv, ok := errorTypeAttr(status, panicked); ok {
					kvs = append(kvs, kv)
				}
				if cfg.scheme != "" {
					kvs = append(kvs, schemeAttr(cfg.scheme))
				}
				dur.Record(rc, time.Since(start).Seconds(),
					metric.WithAttributeSet(attribute.NewSet(kvs...)))
			}
			defer func() {
				if !finished {
					record(true)
				}
			}()
			err = next(ctx)
			finished = true
			record(false)
			return err
		}
	}, nil
}
