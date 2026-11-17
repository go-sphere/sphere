package telemetry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/httpxmock"
	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/server/middleware/requestid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/embedded"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// scriptCtx wraps the mock context so a test can script what an engine would
// report: the matched pattern before and after dispatch, the response status,
// the request context and the header keys the carrier asks for.
// baseCtx gives the embedded httpx.Context a field name that does not clash
// with the Context method scriptCtx overrides.
type baseCtx = httpx.Context

type scriptCtx struct {
	baseCtx
	m                     *httpxmock.Context
	fullBefore, fullAfter string
	dispatched            bool
	status                int
	committed             bool
	reqCtx                context.Context
	headerKeys            []string
}

func (c *scriptCtx) FullPath() string {
	if c.dispatched {
		return c.fullAfter
	}
	return c.fullBefore
}
func (c *scriptCtx) StatusCode() int { return c.status }
func (c *scriptCtx) Committed() bool { return c.committed }
func (c *scriptCtx) Context() context.Context {
	if c.reqCtx != nil {
		return c.reqCtx
	}
	return c.m.Context()
}
func (c *scriptCtx) SetContext(ctx context.Context) {
	c.reqCtx = ctx
	c.m.SetContext(ctx)
}
func (c *scriptCtx) Header(k string) string {
	c.headerKeys = append(c.headerKeys, k)
	return c.m.Header(k)
}

func newScript(method, target string, opts ...httpxmock.Option) *scriptCtx {
	m := httpxmock.NewRequest(method, target, nil, opts...)
	return &scriptCtx{baseCtx: m, m: m, committed: true, status: 200}
}

// run applies mw to a handler that marks the dispatch as done and returns err.
func (c *scriptCtx) run(mw httpx.Middleware, err error) error {
	return mw(func(httpx.Context) error { c.dispatched = true; return err })(c)
}

func TestClientCancelDoesNotFailSpan(t *testing.T) {
	cancelErr := fmt.Errorf("query: %w", context.Canceled)
	for _, c := range []struct {
		name     string
		canceled bool
		code     codes.Code
		errs     int
	}{
		{"client went away", true, codes.Unset, 0},
		{"unrelated context.Canceled", false, codes.Error, 1},
	} {
		p, mws := traceOnly(t)
		reqCtx, cancel := context.WithCancel(context.Background())
		if c.canceled {
			cancel()
		}
		sc := newScript("GET", "/x", httpxmock.WithContext(reqCtx))
		sc.committed = false
		if err := sc.run(mws[0], cancelErr); err == nil {
			t.Fatal("error swallowed")
		}
		s := only(t, p)
		if s.code != c.code || len(s.errs) != c.errs || s.ends != 1 {
			t.Errorf("%s: code=%v errs=%d ends=%d", c.name, s.code, len(s.errs), s.ends)
		}
		cancel()
	}
}

// TestClientCancelDoesNotMarkMetricError pins that the duration metric agrees
// with the span on a client disconnect: the status is still recorded, but no
// error.type, so aborted requests do not count as server errors.
func TestClientCancelDoesNotMarkMetricError(t *testing.T) {
	cancelErr := fmt.Errorf("query: %w", context.Canceled)
	for _, c := range []struct {
		name      string
		canceled  bool
		wantError bool
	}{
		{"client went away", true, false},
		{"unrelated context.Canceled", false, true},
	} {
		mp := newMP()
		reqCtx, cancel := context.WithCancel(context.Background())
		if c.canceled {
			cancel()
		}
		sc := newScript("GET", "/x", httpxmock.WithContext(reqCtx), httpxmock.WithFullPath("/x"))
		sc.committed = false
		if err := sc.run(mustMetrics(t, WithMeterProvider(mp)), cancelErr); err == nil {
			t.Fatal("error swallowed")
		}
		attrs := setOf(mp.hist.sets[0])
		if attrs[keyStatusCode].AsInt64() != 500 {
			t.Errorf("%s: status = %v, want 500", c.name, attrs[keyStatusCode])
		}
		if _, ok := attrs[keyErrorType]; ok != c.wantError {
			t.Errorf("%s: error.type present = %v, want %v", c.name, ok, c.wantError)
		}
		cancel()
	}
}

func TestMetricsWithCanceledContextStillBalanced(t *testing.T) {
	mp := newMP()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sc := newScript("GET", "/x", httpxmock.WithContext(ctx), httpxmock.WithFullPath("/x"))
	if err := sc.run(mustMetrics(t, WithMeterProvider(mp)), nil); err != nil {
		t.Fatal(err)
	}
	if len(mp.hist.sets) != 1 || mp.ctr.sum() != 0 || len(mp.ctr.adds) != 2 {
		t.Fatalf("records=%d active=%v", len(mp.hist.sets), mp.ctr.adds)
	}
}

func TestStatusOutsideRangeIsOmitted(t *testing.T) {
	for _, status := range []int{0, 99, 600, 700} {
		p, mws := traceOnly(t)
		mp := newMP()
		sc := newScript("GET", "/x", httpxmock.WithFullPath("/x"))
		sc.status = status
		if err := sc.run(mws[0], nil); err != nil {
			t.Fatal(err)
		}
		if err := sc.run(mustMetrics(t, WithMeterProvider(mp)), nil); err != nil {
			t.Fatal(err)
		}
		if _, ok := only(t, p).attrs[keyStatusCode]; ok {
			t.Errorf("status %d: span has status code", status)
		}
		if _, ok := setOf(mp.hist.sets[0])[keyStatusCode]; ok {
			t.Errorf("status %d: metric has status code", status)
		}
	}
}

func TestNonStandardMethodIsNormalized(t *testing.T) {
	p, mws := traceOnly(t)
	sc := newScript("FOO", "/x")
	if err := sc.run(mws[0], nil); err != nil {
		t.Fatal(err)
	}
	s := only(t, p)
	if s.attrs[keyMethod].AsString() != "_OTHER" || s.attrs[keyMethodOriginal].AsString() != "FOO" || s.name != "_OTHER" {
		t.Fatalf("name=%q attrs=%v", s.name, s.attrs)
	}
}

func TestPublicEndpointStartsNewRootWithLink(t *testing.T) {
	p, mws := traceOnly(t, WithPublicEndpoint())
	// The base context already carries a span, as behind another instrumented
	// layer; a public endpoint must not parent to it either.
	local := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{9}, SpanID: trace.SpanID{9}, TraceFlags: trace.FlagsSampled,
	}))
	sc := newScript("GET", "/x", httpxmock.WithHeader("Traceparent", tp), httpxmock.WithContext(local))
	if err := sc.run(mws[0], nil); err != nil {
		t.Fatal(err)
	}
	s := only(t, p)
	if s.parent.IsValid() {
		t.Fatalf("public span has remote parent %v", s.parent)
	}
	if len(s.links) != 1 || s.links[0].SpanContext.TraceID().String() != "0af7651916cd43dd8448eb211c80319c" {
		t.Fatalf("links = %v", s.links)
	}
}

func TestURLSchemeOnSpan(t *testing.T) {
	p, mws := traceOnly(t, WithURLScheme("https"))
	if err := newScript("GET", "/x").run(mws[0], nil); err != nil {
		t.Fatal(err)
	}
	if got := only(t, p).attrs["url.scheme"].AsString(); got != "https" {
		t.Fatalf("url.scheme = %q", got)
	}
}

func TestRouteResolvedDuringDispatch(t *testing.T) {
	p, mws := traceOnly(t)
	sc := newScript("GET", "/items/7")
	sc.fullAfter = "/items/:id"
	if err := sc.run(mws[0], nil); err != nil {
		t.Fatal(err)
	}
	s := only(t, p)
	if s.name != "GET /items/:id" || s.attrs[keyRoute].AsString() != "/items/:id" {
		t.Fatalf("name=%q attrs=%v", s.name, s.attrs)
	}
}

// An engine such as hertz leaves FullPath empty for 404/405 (see the hertz
// RequestContext: fullPath is set only when a handler chain matched), while
// Path is the raw request path. The route must stay low-cardinality.
func TestEmptyFullPathOn404IsUnmatched(t *testing.T) {
	p, mws := traceOnly(t)
	mp := newMP()
	mws = append(mws, mustMetrics(t, WithMeterProvider(mp)))
	for i := range 5 {
		sc := newScript("GET", fmt.Sprintf("/random/%d", i))
		sc.status = http.StatusNotFound
		if err := httpx.ComposeMiddleware(func(httpx.Context) error { sc.dispatched = true; return nil }, mws)(sc); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range p.spans {
		if _, ok := s.attrs[keyRoute]; ok || s.name != "GET" {
			t.Fatalf("name=%q attrs=%v", s.name, s.attrs)
		}
	}
	for _, set := range mp.hist.sets {
		if setOf(set)[keyRoute].AsString() != UnmatchedRoute {
			t.Fatalf("attrs = %v", setOf(set))
		}
	}
}

func TestDurationIsRecordedInSeconds(t *testing.T) {
	mp := newMP()
	sc := newScript("GET", "/x", httpxmock.WithFullPath("/x"))
	mw := mustMetrics(t, WithMeterProvider(mp))
	err := mw(func(httpx.Context) error { time.Sleep(20 * time.Millisecond); return nil })(sc)
	if err != nil {
		t.Fatal(err)
	}
	if v := mp.hist.vals[0]; v < 0.02 || v > 5 {
		t.Fatalf("duration = %v, want seconds", v)
	}
}

func TestCarrierPassesKeyVerbatim(t *testing.T) {
	sc := newScript("GET", "/x", httpxmock.WithHeader("Traceparent", tp))
	_ = headerCarrier{sc}.Get("traceparent")
	if len(sc.headerKeys) != 1 || sc.headerKeys[0] != "traceparent" {
		t.Fatalf("keys asked = %v", sc.headerKeys)
	}
}

func TestTraceAttrsValues(t *testing.T) {
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1}, SpanID: trace.SpanID{2}, TraceFlags: trace.FlagsSampled,
	})
	a := TraceAttrs(trace.ContextWithSpanContext(context.Background(), sc))
	if a[0].Value.String() != sc.TraceID().String() || a[1].Value.String() != sc.SpanID().String() {
		t.Fatalf("attrs = %v", a)
	}
}

// The upstream traceparent must surface as trace_id in the request-scoped
// logs, next to the request id, with the server span's own span_id.
func TestLogTraceIDFollowsUpstreamTraceparent(t *testing.T) {
	be := &capture{}
	lg := log.NewLogger(be)
	p, mws := traceOnly(t)
	mws = append(mws, requestid.New())
	serve(t, mws, func(r httpx.Router) {
		r.GET("/x", func(ctx httpx.Context) error {
			lg.InfoContext(ctx.Context(), "in handler")
			return nil
		})
	}, "GET", "/x", map[string]string{"Traceparent": tp})
	if len(be.entries) != 1 {
		t.Fatalf("entries = %d", len(be.entries))
	}
	got := map[string]string{}
	for _, a := range be.entries[0] {
		got[a.Key] = a.Value.String()
	}
	if got[TraceIDKey] != "0af7651916cd43dd8448eb211c80319c" || got[SpanIDKey] != only(t, p).sc.SpanID().String() || got[requestid.LogKey] == "" {
		t.Fatalf("log attrs = %v", got)
	}
}

var _ propagation.TextMapCarrier = headerCarrier{}

func TestNilOptionsAreSkipped(t *testing.T) {
	if got := newConfig([]Option{nil, WithURLScheme("https"), nil}).scheme; got != "https" {
		t.Fatalf("scheme = %q, want https", got)
	}
}

type nonRecordingSpan struct {
	tracenoop.Span
	mutated bool
	ended   int
}

func (s *nonRecordingSpan) IsRecording() bool                       { return false }
func (s *nonRecordingSpan) SetName(string)                          { s.mutated = true }
func (s *nonRecordingSpan) SetAttributes(...attribute.KeyValue)     { s.mutated = true }
func (s *nonRecordingSpan) SetStatus(codes.Code, string)            { s.mutated = true }
func (s *nonRecordingSpan) RecordError(error, ...trace.EventOption) { s.mutated = true }
func (s *nonRecordingSpan) End(...trace.SpanEndOption)              { s.ended++ }

type nonRecordingTP struct {
	embedded.TracerProvider
	span *nonRecordingSpan
}

func (p *nonRecordingTP) Tracer(string, ...trace.TracerOption) trace.Tracer {
	return nonRecordingTracer{span: p.span}
}

type nonRecordingTracer struct {
	embedded.Tracer
	span *nonRecordingSpan
}

func (tr nonRecordingTracer) Start(ctx context.Context, _ string, _ ...trace.SpanStartOption) (context.Context, trace.Span) {
	return trace.ContextWithSpan(ctx, tr.span), tr.span
}

func TestNonRecordingSpanSkipsPostHandlerAttributes(t *testing.T) {
	sp := &nonRecordingSpan{}
	mw := NewTracing(WithTracerProvider(&nonRecordingTP{span: sp}))
	sc := newScript("GET", "/items/1")
	sc.fullAfter = "/items/:id"
	if err := sc.run(mw, errors.New("fail")); err == nil {
		t.Fatal("expected error")
	}
	if sp.mutated {
		t.Fatal("post-handler span work ran on non-recording span")
	}
	if sp.ended != 1 {
		t.Fatalf("ended = %d, want 1", sp.ended)
	}
}

func TestClientCanceledUsesInboundReqCtxWhenHandlerReplacesContext(t *testing.T) {
	cancelErr := fmt.Errorf("downstream: %w", context.Canceled)
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	t.Run("active inbound ctx is treated as server error even if handler sets canceled ctx", func(t *testing.T) {
		p, mws := traceOnly(t)
		mp := newMP()
		mmw := mustMetrics(t, WithMeterProvider(mp))
		sc := newScript("GET", "/x", httpxmock.WithContext(context.Background()), httpxmock.WithFullPath("/x"))
		sc.committed = false
		err := httpx.ComposeMiddleware(func(c httpx.Context) error {
			c.SetContext(canceledCtx)
			return cancelErr
		}, []httpx.Middleware{mws[0], mmw})(sc)
		if err == nil {
			t.Fatal("expected error")
		}
		if s := only(t, p); s.code != codes.Error {
			t.Fatalf("span code = %v, want Error", s.code)
		}
		if _, ok := setOf(mp.hist.sets[0])[keyErrorType]; !ok {
			t.Fatal("expected error.type on metric when inbound ctx was active")
		}
	})

	t.Run("canceled inbound ctx stays non-error even if handler resets ctx", func(t *testing.T) {
		p, mws := traceOnly(t)
		mp := newMP()
		mmw := mustMetrics(t, WithMeterProvider(mp))
		sc := newScript("GET", "/x", httpxmock.WithContext(canceledCtx), httpxmock.WithFullPath("/x"))
		sc.committed = false
		err := httpx.ComposeMiddleware(func(c httpx.Context) error {
			c.SetContext(context.Background())
			return cancelErr
		}, []httpx.Middleware{mws[0], mmw})(sc)
		if err == nil {
			t.Fatal("expected error")
		}
		if s := only(t, p); s.code != codes.Unset {
			t.Fatalf("span code = %v, want Unset", s.code)
		}
		if _, ok := setOf(mp.hist.sets[0])[keyErrorType]; ok {
			t.Fatal("unexpected error.type on metric when inbound ctx was canceled")
		}
	})
}
