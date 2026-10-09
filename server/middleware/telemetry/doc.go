// Package telemetry provides OpenTelemetry tracing and metrics middleware for
// httpx servers, built on the OpenTelemetry API only. It never installs an SDK,
// exporter, provider or propagator, and has no init side effects.
//
// # What the application must do
//
// The otel globals are no-ops until the application installs an SDK, and the
// global propagator is a no-op until set. Without these the middleware works
// but records nothing and every request is a new root span:
//
//	otel.SetTracerProvider(tp)
//	otel.SetMeterProvider(mp)
//	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
//		propagation.TraceContext{}, propagation.Baggage{}))
//
// Alternatively pass providers and propagators with [WithTracerProvider],
// [WithMeterProvider] and [WithPropagators]. The SDK, views and bucket
// configuration, the /metrics endpoint, exporters, and operational endpoints
// such as health checks and pprof are outside this package.
//
// # Order
//
// Register outermost first:
//
//	engine.Use(
//		telemetry.NewTracing(),   // spans every layer; ends the span on panic
//		metrics,                  // inside tracing so recordings carry the span
//		requestid.New(),
//		logger.Log(lg),
//		logger.RecoveryLog(lg, true), // inside tracing, inside Log
//	)
//
// Recovery sits inside tracing: RecoveryLog commits a 500, so the span is
// marked Error with no exception event; RecoveryLogErr also returns a
// *logger.PanicError, so the span records the panic as an event. A panic that
// nothing recovers (including http.ErrAbortHandler) still ends the span and
// records the duration before it continues.
//
// # Logs
//
// By default NewTracing attaches trace_id and span_id of the server span to the
// request context with log.ContextWithAttrs, so entries written with the
// *Context methods carry them. For the id of the current (child) span use
// [WithoutLogAttrs] plus log.WrapBackendWithContextMerge with [TraceAttrs];
// combining both modes would let the stale server span_id win.
//
// # Limits
//
// Only the first line of a multi-line baggage header is read. Not collected:
// client address, url.query, headers, bodies, and scheme, host or protocol
// inferred from forwarding headers. Request errors recorded as span events
// carry err.Error(); use [WithoutErrorEvents] when that is sensitive.
package telemetry
