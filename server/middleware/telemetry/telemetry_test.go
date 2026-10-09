package telemetry

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/httpxmock"
	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/server/httpz"
	"github.com/go-sphere/sphere/server/middleware/logger"
	"github.com/go-sphere/sphere/server/middleware/requestid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const tp = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"

func routes(r httpx.Router) {
	ok := func(ctx httpx.Context) error { return ctx.JSON(200, map[string]string{"ok": "1"}) }
	r.GET("/api/v1/users/:id", ok)
	r.GET("/forbidden", func(httpx.Context) error { return httpx.NewForbiddenError("no") })
	r.GET("/boom", func(httpx.Context) error { return errBoom })
	r.GET("/panic", func(httpx.Context) error { panic("kaboom") })
	r.GET("/abort", func(httpx.Context) error { panic(http.ErrAbortHandler) })
	r.GET("/committed", func(ctx httpx.Context) error {
		_ = ctx.JSON(200, "x")
		return errBoom
	})
	r.GET("/with", httpz.WithJson(func(httpx.Context) (int, error) { return 0, errBoom }))
}

func traceOnly(t *testing.T, opts ...Option) (*fakeTracerProvider, []httpx.Middleware) {
	t.Helper()
	p := &fakeTracerProvider{}
	return p, []httpx.Middleware{NewTracing(append([]Option{WithTracerProvider(p), WithPropagators(propagation.TraceContext{})}, opts...)...)}
}

func only(t *testing.T, p *fakeTracerProvider) *fakeSpan {
	t.Helper()
	if len(p.spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(p.spans))
	}
	return p.spans[0]
}

func TestMatchedRouteSpan(t *testing.T) {
	p, mws := traceOnly(t)
	serve(t, mws, routes, "GET", "/api/v1/users/42", map[string]string{"User-Agent": "ua"})
	s := only(t, p)
	if s.name != "GET /api/v1/users/:id" || s.kind != trace.SpanKindServer {
		t.Fatalf("name=%q kind=%v", s.name, s.kind)
	}
	if s.attrs[keyRoute].AsString() != "/api/v1/users/:id" || s.attrs[keyStatusCode].AsInt64() != 200 {
		t.Fatalf("attrs = %v", s.attrs)
	}
	if s.attrs["url.path"].AsString() != "/api/v1/users/42" || s.attrs["user_agent.original"].AsString() != "ua" {
		t.Fatalf("attrs = %v", s.attrs)
	}
	if s.ends != 1 || s.code != codes.Unset {
		t.Fatalf("ends=%d code=%v", s.ends, s.code)
	}
}

func TestUnmatchedRouteSpanIsLowCardinality(t *testing.T) {
	p, mws := traceOnly(t)
	for i := range 20 {
		serve(t, mws, routes, "GET", "/nope/"+strings.Repeat("x", i+1), nil)
	}
	for _, s := range p.spans {
		if s.name != "GET" {
			t.Fatalf("name = %q", s.name)
		}
		if _, ok := s.attrs[keyRoute]; ok {
			t.Fatal("unmatched span has http.route")
		}
	}
}

func TestStatusMapping(t *testing.T) {
	cases := []struct {
		path   string
		status int64
		code   codes.Code
		errs   int
	}{
		{"/forbidden", 403, codes.Unset, 0},
		{"/boom", 500, codes.Error, 1},
		{"/committed", 200, codes.Unset, 0},
		{"/with", 500, codes.Unset, 0}, // WithJson renders and returns nil; its status is on the response
	}
	for _, c := range cases {
		p, mws := traceOnly(t)
		serve(t, mws, routes, "GET", c.path, nil)
		s := only(t, p)
		if c.path == "/with" {
			c.code = codes.Error
		}
		if s.attrs[keyStatusCode].AsInt64() != c.status || s.code != c.code || len(s.errs) != c.errs {
			t.Errorf("%s: status=%v code=%v errs=%d", c.path, s.attrs[keyStatusCode], s.code, len(s.errs))
		}
		if s.code == codes.Error && s.desc != "Internal Server Error" {
			t.Errorf("%s: desc = %q", c.path, s.desc)
		}
	}
}

func TestWithoutErrorEvents(t *testing.T) {
	p, mws := traceOnly(t, WithoutErrorEvents())
	serve(t, mws, routes, "GET", "/boom", nil)
	if s := only(t, p); s.code != codes.Error || len(s.errs) != 0 {
		t.Fatalf("code=%v errs=%d", s.code, len(s.errs))
	}
}

func TestErrorParserIsHonored(t *testing.T) {
	httpz.SetDefaultErrorParser(func(err error) (int32, int32, string) { return 1, 503, "x" })
	t.Cleanup(func() { httpz.SetDefaultErrorParser(httpz.ParseError) })
	p, mws := traceOnly(t)
	serve(t, mws, routes, "GET", "/boom", nil)
	if s := only(t, p); s.attrs[keyStatusCode].AsInt64() != 503 || s.code != codes.Error {
		t.Fatalf("attrs=%v code=%v", s.attrs, s.code)
	}
}

func servePanic(t *testing.T, mws []httpx.Middleware, path string) (rec any) {
	t.Helper()
	defer func() { rec = recover() }()
	serve(t, mws, routes, "GET", path, nil)
	return nil
}

func TestUnrecoveredPanicEndsSpanAndPropagates(t *testing.T) {
	for _, path := range []string{"/panic", "/abort"} {
		p, mws := traceOnly(t)
		if servePanic(t, mws, path) == nil {
			t.Fatalf("%s: panic was swallowed", path)
		}
		s := only(t, p)
		if s.ends != 1 || s.code != codes.Error || s.attrs[keyErrorType].AsString() != "panic" {
			t.Errorf("%s: ends=%d code=%v attrs=%v", path, s.ends, s.code, s.attrs)
		}
	}
}

type nopLog struct{}

func (nopLog) Debug(string, ...log.Attr) {}
func (nopLog) Info(string, ...log.Attr)  {}
func (nopLog) Warn(string, ...log.Attr)  {}
func (nopLog) Error(string, ...log.Attr) {}

func TestRecoveryInsideTracingAndMetrics(t *testing.T) {
	for _, withErr := range []bool{false, true} {
		p, mws := traceOnly(t)
		mp := newMP()
		rec := logger.RecoveryLog(nopLog{}, false)
		if withErr {
			rec = logger.RecoveryLogErr(nopLog{}, false)
		}
		mws = append(mws, mustMetrics(t, WithMeterProvider(mp)), rec)
		if r := servePanic(t, mws, "/panic"); r != nil {
			t.Fatalf("panic escaped recovery: %v", r)
		}
		s := only(t, p)
		if s.code != codes.Error || s.attrs[keyStatusCode].AsInt64() != 500 {
			t.Errorf("withErr=%v: code=%v attrs=%v", withErr, s.code, s.attrs)
		}
		if withErr != (len(s.errs) == 1) || (withErr && !strings.HasPrefix(s.errs[0].Error(), "panic:")) {
			t.Errorf("withErr=%v: errs=%v", withErr, s.errs)
		}
		m := setOf(mp.hist.sets[0])
		if m[keyStatusCode].AsInt64() != 500 || m[keyErrorType].AsString() != "500" {
			t.Errorf("withErr=%v: metric attrs = %v", withErr, m)
		}
	}
}

func TestPropagation(t *testing.T) {
	p0, mws0 := traceOnly(t)
	serve(t, mws0, routes, "GET", "/api/v1/users/1", map[string]string{"Traceparent": tp})
	if s := only(t, p0); s.parent.TraceID().String() != "0af7651916cd43dd8448eb211c80319c" || !s.parent.IsRemote() {
		t.Errorf("parent = %v", s.parent)
	}

	p, mws := traceOnly(t, WithPublicEndpoint())
	serve(t, mws, routes, "GET", "/api/v1/users/1", map[string]string{"Traceparent": tp})
	s := only(t, p)
	if s.parent.IsValid() || len(s.links) != 1 || s.links[0].SpanContext.TraceID().String() != "0af7651916cd43dd8448eb211c80319c" {
		t.Errorf("public: parent=%v links=%v", s.parent, s.links)
	}

	// Default global propagator is a no-op: a new root, no panic.
	q := &fakeTracerProvider{}
	serve(t, []httpx.Middleware{NewTracing(WithTracerProvider(q))}, routes, "GET", "/api/v1/users/1", map[string]string{"Traceparent": tp})
	if only(t, q).parent.IsValid() {
		t.Error("global no-op propagator produced a parent")
	}
}

type capture struct {
	mu      sync.Mutex
	entries [][]log.Attr
}

func (c *capture) Log(_ context.Context, _ log.Level, _ string, attrs ...log.Attr) {
	c.mu.Lock()
	c.entries = append(c.entries, attrs)
	c.mu.Unlock()
}

func countKey(attrs []log.Attr, key string) int {
	n := 0
	for _, a := range attrs {
		if a.Key == key {
			n++
		}
	}
	return n
}

func TestLogCorrelation(t *testing.T) {
	be := &capture{}
	lg := log.NewLogger(be)
	p, mws := traceOnly(t)
	mws = append(mws, requestid.New(), logger.Log(lg), logger.RecoveryLog(lg, false))
	serve(t, mws, func(r httpx.Router) {
		r.GET("/x", func(ctx httpx.Context) error {
			if trace.SpanFromContext(ctx.Context()) != trace.Span(p.spans[0]) {
				t.Error("ctx does not carry the span")
			}
			lg.InfoContext(ctx.Context(), "in handler")
			return nil
		})
	}, "GET", "/x", nil)
	if len(be.entries) != 2 {
		t.Fatalf("entries = %d", len(be.entries))
	}
	for _, e := range be.entries {
		for _, k := range []string{TraceIDKey, SpanIDKey, requestid.LogKey} {
			if countKey(e, k) != 1 {
				t.Errorf("key %s count = %d in %v", k, countKey(e, k), e)
			}
		}
	}
}

func TestLogCorrelationViaExtractor(t *testing.T) {
	be := &capture{}
	lg := log.NewLogger(log.WrapBackendWithContextMerge(be, TraceAttrs))
	_, mws := traceOnly(t, WithoutLogAttrs())
	mws = append(mws, logger.Log(lg))
	serve(t, mws, routes, "GET", "/api/v1/users/1", nil)
	if len(be.entries) != 1 || countKey(be.entries[0], TraceIDKey) != 1 || countKey(be.entries[0], SpanIDKey) != 1 {
		t.Fatalf("entries = %v", be.entries)
	}
}

func TestTraceAttrs(t *testing.T) {
	if TraceAttrs(context.Background()) != nil || TraceID(context.Background()) != "" || SpanID(context.Background()) != "" {
		t.Fatal("empty ctx should give no attrs")
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1}, SpanID: trace.SpanID{2}, TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	a := TraceAttrs(ctx)
	if len(a) != 2 || a[0].Key != "trace_id" || a[1].Key != "span_id" || TraceID(ctx) != sc.TraceID().String() || SpanID(ctx) != sc.SpanID().String() {
		t.Fatalf("attrs = %v", a)
	}
}

func TestWithSkip(t *testing.T) {
	p, mws := traceOnly(t, WithSkip(func(c httpx.Context) bool { return c.Path() == "/boom" }))
	mp := newMP()
	mws = append(mws, mustMetrics(t, WithMeterProvider(mp), WithSkip(func(c httpx.Context) bool { return c.Path() == "/boom" })))
	serve(t, mws, routes, "GET", "/boom", nil)
	if len(p.spans) != 0 || len(mp.hist.sets) != 0 || len(mp.ctr.adds) != 0 {
		t.Fatal("skipped request was observed")
	}
}

func TestSpanScopedToRequestAndConcurrent(t *testing.T) {
	p, mws := traceOnly(t)
	mp := newMP()
	mws = append(mws, mustMetrics(t, WithMeterProvider(mp)))
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() { serve(t, mws, routes, "GET", "/api/v1/users/1", nil) })
	}
	wg.Wait()
	if len(p.spans) != 16 || len(mp.hist.sets) != 16 || mp.ctr.sum() != 0 {
		t.Fatalf("spans=%d records=%d active=%d", len(p.spans), len(mp.hist.sets), mp.ctr.sum())
	}
}

func TestCarrier(t *testing.T) {
	c := httpxmock.NewRequest("GET", "/x", nil, httpxmock.WithHeader("Traceparent", tp))
	car := headerCarrier{c}
	if car.Get("traceparent") != tp || car.Get("TRACEPARENT") != tp || car.Get("missing") != "" {
		t.Fatal("Get is not case-insensitive")
	}
	car.Set("x", "y")
	if car.Get("x") != "" {
		t.Fatal("Set must be a no-op")
	}
	if keys := car.Keys(); len(keys) != 1 || !strings.EqualFold(keys[0], "traceparent") {
		t.Fatalf("keys = %v", keys)
	}
}

func TestMetricsAttributes(t *testing.T) {
	mp := newMP()
	mws := []httpx.Middleware{mustMetrics(t, WithMeterProvider(mp))}
	serve(t, mws, routes, "GET", "/api/v1/users/1", nil)
	serve(t, mws, routes, "GET", "/boom", nil)
	serve(t, mws, routes, "GET", "/forbidden", nil)
	want := []map[string]attribute.Value{
		{keyMethod: attribute.StringValue("GET"), keyRoute: attribute.StringValue("/api/v1/users/:id"), keyStatusCode: attribute.Int64Value(200)},
		{keyMethod: attribute.StringValue("GET"), keyRoute: attribute.StringValue("/boom"), keyStatusCode: attribute.Int64Value(500), keyErrorType: attribute.StringValue("500")},
		{keyMethod: attribute.StringValue("GET"), keyRoute: attribute.StringValue("/forbidden"), keyStatusCode: attribute.Int64Value(403)},
	}
	for i, w := range want {
		got := setOf(mp.hist.sets[i])
		if len(got) != len(w) {
			t.Errorf("%d: got %v want %v", i, got, w)
		}
		for k, v := range w {
			if got[k] != v {
				t.Errorf("%d: %s = %v, want %v", i, k, got[k], v)
			}
		}
	}
	if mp.ctr.sum() != 0 || len(mp.ctr.adds) != 6 {
		t.Fatalf("active adds = %v", mp.ctr.adds)
	}
}

func TestMetricsCardinality(t *testing.T) {
	mp := newMP()
	mws := []httpx.Middleware{mustMetrics(t, WithMeterProvider(mp))}
	for i := range 200 {
		serve(t, mws, routes, "GET", "/nope/"+strings.Repeat("a", i+1), nil)
	}
	for _, m := range []string{"FOO", "BAR"} {
		serve(t, mws, routes, m, "/nope", nil)
	}
	seen := map[attribute.Distinct]bool{}
	for _, s := range mp.hist.sets {
		seen[s.Equivalent()] = true
		if setOf(s)[keyRoute].AsString() != UnmatchedRoute {
			t.Fatalf("route = %v", setOf(s)[keyRoute])
		}
	}
	// GET and _OTHER, each with whatever status unmatched routes produce.
	if len(seen) > 2 {
		t.Fatalf("distinct sets = %d", len(seen))
	}
}

func TestMetricsPanicBalanced(t *testing.T) {
	mp := newMP()
	mws := []httpx.Middleware{mustMetrics(t, WithMeterProvider(mp))}
	if servePanic(t, mws, "/panic") == nil {
		t.Fatal("panic swallowed")
	}
	if mp.ctr.sum() != 0 || len(mp.ctr.adds) != 2 {
		t.Fatalf("active adds = %v", mp.ctr.adds)
	}
	if len(mp.hist.sets) != 1 {
		t.Fatalf("records = %d", len(mp.hist.sets))
	}
	m := setOf(mp.hist.sets[0])
	if _, ok := m[keyStatusCode]; ok || m[keyErrorType].AsString() != "panic" {
		t.Fatalf("attrs = %v", m)
	}
}

func TestMetricsDisabledPassThrough(t *testing.T) {
	mp := newMP()
	mw := mustMetrics(t, WithMeterProvider(mp), WithDurationHistogram(nil), WithoutActiveRequests())
	serve(t, []httpx.Middleware{mw}, routes, "GET", "/boom", nil)
	if len(mp.hist.sets) != 0 || len(mp.ctr.adds) != 0 {
		t.Fatal("disabled instruments were used")
	}
	called := false
	h := mw(func(httpx.Context) error { called = true; return nil })
	_ = h(httpxmock.NewRequest("GET", "/x", nil))
	if !called {
		t.Fatal("handler not called")
	}
}

func TestMetricsInjectedInstruments(t *testing.T) {
	h, c := &fakeHist{}, &fakeCounter{}
	mw := mustMetrics(t, WithDurationHistogram(h), WithActiveRequests(c), WithURLScheme("https"))
	serve(t, []httpx.Middleware{mw}, routes, "GET", "/boom", nil)
	if len(h.sets) != 1 || len(c.adds) != 2 || setOf(h.sets[0])["url.scheme"].AsString() != "https" {
		t.Fatalf("hist=%v active=%v", h.sets, c.adds)
	}
}

func TestMetricsInstrumentError(t *testing.T) {
	mp := newMP()
	mp.err = errBoom
	if _, err := NewMetrics(WithMeterProvider(mp)); err == nil {
		t.Fatal("instrument error swallowed")
	}
}

func TestContracts(t *testing.T) {
	if UnmatchedRoute != logger.UnmatchedRoute {
		t.Fatal("UnmatchedRoute drifted from logger.UnmatchedRoute")
	}
	if TraceIDKey != "trace_id" || SpanIDKey != "span_id" {
		t.Fatal("log keys changed")
	}
}

func TestNoopProviderPassThrough(t *testing.T) {
	mws := []httpx.Middleware{NewTracing(), mustMetrics(t)}
	if w := serve(t, mws, routes, "GET", "/api/v1/users/1", nil); w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
}

func (*capture) Sync() error { return nil }

func (c *capture) With(...log.Option) log.Backend { return c }
