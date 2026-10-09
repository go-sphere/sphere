// Package stack assembles the standard engine-level middleware stack in one
// documented, tested order.
//
// [New] returns the layers as []httpx.Middleware and [Apply] registers them on
// an engine. Both are optional helpers: the individual middleware packages
// keep working on their own, and nothing here changes their signatures.
//
// # Order
//
// Outermost first:
//
//  1. telemetry.NewTracing (opt-in with [WithTracing])
//  2. telemetry.NewMetrics (opt-in with [WithMetrics])
//  3. requestid.New (default on; [WithoutRequestID] disables it)
//  4. logger.Log, the access log (needs [WithLogger]; [WithoutAccessLog] disables it)
//  5. logger.RecoveryLog or RecoveryLogErr (default on; [WithoutRecovery] disables it)
//
// A disabled or skipped layer leaves the others in the same relative order.
// The constraints that change behavior, each pinned by an end-to-end test:
//
//   - Recovery is innermost of the stack, inside Tracing and Metrics. It
//     commits a 500 before the panic unwinds through them, so the span is
//     marked Error with status 500 and the duration histogram records
//     http.response.status_code=500. Outside them, Tracing and Metrics only see
//     an unrecovered panic (error.type "panic", no status). With
//     [WithRecoveryError] the span also records the panic as an event.
//   - Recovery is inside Log: it finishes the request as 500, so the access log
//     still writes one entry (status=500) for the panicking request. With
//     Recovery outside Log the panic unwinds past Log and the request has no
//     access entry.
//   - Metrics is directly inside Tracing: it captures the request context when
//     the request starts, so recordings carry the span context only if Tracing
//     already put the span there.
//
// The rest is convention, widest scope first, pinned only by the layer order
// itself: Tracing outermost so the span covers every other layer, then
// Metrics, then RequestID so the ID exists for every layer that logs and the
// response header is set before an inner layer can reject the request, then
// Log. The log fields themselves do not depend on that relative order:
// logger.Log reads the request context after the chain, so request_id,
// trace_id and span_id added by any inner layer reach the access entry.
//
// # Engine level and group level
//
// Register the stack, and CORS, with engine.Use ([Apply] does this). The
// engine-level chain also runs for requests no route matched (404, 405), the
// group-level chain does not (httpx.MiddlewareScope). Engine-level layers are
// always outside group-level layers, so the stack can never be reordered by
// what a group adds. Use rules:
//
//   - Engine level: this stack, CORS (a preflight for an unmatched path still
//     needs its headers; register it after [Apply] so preflights are logged
//     and traced), and anything that must observe or protect unmatched paths.
//   - Group level: authentication, permission checks, rate limiting, online
//     tracking and per-operation selectors (server/middleware/selector).
//     They depend on the matched route or on a principal, and should not
//     reject or count unmatched paths. They run inside the stack, so their
//     errors are traced, counted and logged.
//   - Call [Apply] before registering routes: middleware added with Use only
//     reaches routes registered afterwards.
//
// # Dependencies
//
// The access log needs a logger: without [WithLogger] it is skipped. Recovery
// never needs one to work; without [WithLogger] it logs panics through a
// log.NewStdioBackend logger so a panicking handler still gets a 500 instead
// of a dropped connection. Telemetry layers use the global OpenTelemetry
// providers unless given providers in their telemetry.Option values; see the
// telemetry package for what the application must install.
package stack
