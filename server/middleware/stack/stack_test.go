package stack

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/stdx"
	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/server/middleware/requestid"
	"github.com/go-sphere/sphere/server/middleware/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/embedded"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// Minimal OpenTelemetry fakes: the API modules only, no SDK.

type fakeSpan struct {
	tracenoop.Span
	mu    sync.Mutex
	name  string
	attrs map[string]attribute.Value
	code  codes.Code
	ends  int
	sc    trace.SpanContext
}

func (s *fakeSpan) SpanContext() trace.SpanContext { return s.sc }
func (s *fakeSpan) IsRecording() bool              { return true }
func (s *fakeSpan) SetName(n string)               { s.mu.Lock(); s.name = n; s.mu.Unlock() }
func (s *fakeSpan) SetAttributes(kv ...attribute.KeyValue) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range kv {
		s.attrs[string(a.Key)] = a.Value
	}
}
func (s *fakeSpan) SetStatus(c codes.Code, _ string) { s.mu.Lock(); s.code = c; s.mu.Unlock() }
func (s *fakeSpan) End(...trace.SpanEndOption)       { s.mu.Lock(); s.ends++; s.mu.Unlock() }

type fakeTracerProvider struct {
	embedded.TracerProvider
	mu    sync.Mutex
	spans []*fakeSpan
}

func (p *fakeTracerProvider) Tracer(string, ...trace.TracerOption) trace.Tracer {
	return &fakeTracer{p: p}
}

type fakeTracer struct {
	embedded.Tracer
	p *fakeTracerProvider
}

func (t *fakeTracer) Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	cfg := trace.NewSpanStartConfig(opts...)
	p := t.p
	p.mu.Lock()
	defer p.mu.Unlock()
	s := &fakeSpan{name: name, attrs: map[string]attribute.Value{}}
	for _, a := range cfg.Attributes() {
		s.attrs[string(a.Key)] = a.Value
	}
	s.sc = trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1, 2, 3}, SpanID: trace.SpanID{byte(len(p.spans) + 1)}, TraceFlags: trace.FlagsSampled,
	})
	p.spans = append(p.spans, s)
	return trace.ContextWithSpan(ctx, s), s
}

type fakeHist struct {
	metricnoop.Float64Histogram
	mu   sync.Mutex
	sets []attribute.Set
	ctxs []context.Context
}

func (h *fakeHist) Enabled(context.Context) bool { return true }
func (h *fakeHist) Record(ctx context.Context, _ float64, opts ...metric.RecordOption) {
	cfg := metric.NewRecordConfig(opts)
	h.mu.Lock()
	h.sets = append(h.sets, cfg.Attributes())
	h.ctxs = append(h.ctxs, ctx)
	h.mu.Unlock()
}

type fakeMeterProvider struct {
	metricnoop.MeterProvider
	hist *fakeHist
	err  error
}

func (p *fakeMeterProvider) Meter(string, ...metric.MeterOption) metric.Meter {
	return &fakeMeter{p: p}
}

type fakeMeter struct {
	metricnoop.Meter
	p *fakeMeterProvider
}

func (m *fakeMeter) Float64Histogram(string, ...metric.Float64HistogramOption) (metric.Float64Histogram, error) {
	return m.p.hist, m.p.err
}

type capture struct {
	mu      sync.Mutex
	entries []entry
}

type entry struct {
	level log.Level
	msg   string
	attrs map[string]string
}

func (c *capture) Log(_ context.Context, level log.Level, msg string, attrs ...log.Attr) {
	m := map[string]string{}
	for _, a := range attrs {
		m[a.Key] = a.Value.String()
	}
	c.mu.Lock()
	c.entries = append(c.entries, entry{level, msg, m})
	c.mu.Unlock()
}
func (c *capture) Sync() error                    { return nil }
func (c *capture) With(...log.Option) log.Backend { return c }

func routes(r httpx.Router) {
	r.GET("/ok", func(ctx httpx.Context) error { return ctx.NoContent(http.StatusNoContent) })
	r.GET("/items/:id", func(ctx httpx.Context) error { panic("boom") })
}

// serve registers the stack from opts on a stdx engine, then routes, and
// serves one request.
func serve(t *testing.T, opts []Option, setup func(httpx.Engine), target string) *httptest.ResponseRecorder {
	t.Helper()
	app := stdx.New()
	if err := Apply(app, opts...); err != nil {
		t.Fatal(err)
	}
	if setup != nil {
		setup(app)
	}
	routes(app.Group(""))
	w := httptest.NewRecorder()
	app.(http.Handler).ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
	return w
}

func names(t *testing.T, opts ...Option) []string {
	t.Helper()
	layers, err := plan(opts)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, l := range layers {
		out = append(out, l.name)
	}
	return out
}

func TestPlanOrder(t *testing.T) {
	lg := log.NewLogger(&capture{})
	mp := &fakeMeterProvider{hist: &fakeHist{}}
	tests := []struct {
		name string
		opts []Option
		want []string
	}{
		{"full", []Option{WithLogger(lg), WithTracing(), WithMetrics(telemetry.WithMeterProvider(mp), telemetry.WithoutActiveRequests())},
			[]string{layerTracing, layerMetrics, layerRequestID, layerAccessLog, layerRecovery}},
		{"defaults", nil, []string{layerRequestID, layerRecovery}},
		{"logger", []Option{WithLogger(lg)}, []string{layerRequestID, layerAccessLog, layerRecovery}},
		{"no access log", []Option{WithLogger(lg), WithoutAccessLog()}, []string{layerRequestID, layerRecovery}},
		{"no requestid", []Option{WithLogger(lg), WithoutRequestID()}, []string{layerAccessLog, layerRecovery}},
		{"no recovery", []Option{WithLogger(lg), WithoutRecovery(), WithTracing()}, []string{layerTracing, layerRequestID, layerAccessLog}},
		{"requestid options keep it on", []Option{WithRequestID(requestid.WithAlwaysGenerate())}, []string{layerRequestID, layerRecovery}},
		{"everything off", []Option{WithoutRequestID(), WithoutRecovery()}, nil},
	}
	for _, tt := range tests {
		if got := names(t, tt.opts...); !slices.Equal(got, tt.want) {
			t.Errorf("%s: layers = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestNewReturnsOneMiddlewarePerLayer(t *testing.T) {
	mws, err := New(WithLogger(log.NewLogger(&capture{})))
	if err != nil || len(mws) != 3 {
		t.Fatalf("len=%d err=%v", len(mws), err)
	}
}

// TestPanicIsTracedCountedAndLogged pins the order end to end: a recovered
// panic is a 500 span and a 500 duration record (Recovery inside Tracing and
// Metrics), produces an access entry (Recovery inside Log), the duration
// record carries the span context (Metrics inside Tracing), and both the
// access entry and the panic entry carry request_id, trace_id and span_id.
func TestPanicIsTracedCountedAndLogged(t *testing.T) {
	be := &capture{}
	tp := &fakeTracerProvider{}
	mp := &fakeMeterProvider{hist: &fakeHist{}}
	w := serve(t, []Option{
		WithLogger(log.NewLogger(be)),
		WithTracing(telemetry.WithTracerProvider(tp)),
		WithMetrics(telemetry.WithMeterProvider(mp), telemetry.WithoutActiveRequests()),
	}, nil, "/items/7")

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	rid := w.Header().Get(requestid.DefaultHeader)
	if rid == "" {
		t.Fatal("response has no request id header")
	}

	if len(tp.spans) != 1 {
		t.Fatalf("spans = %d", len(tp.spans))
	}
	s := tp.spans[0]
	if s.ends != 1 || s.code != codes.Error || s.attrs["http.response.status_code"].AsInt64() != 500 {
		t.Errorf("span: ends=%d code=%v attrs=%v", s.ends, s.code, s.attrs)
	}
	if s.attrs["error.type"].AsString() == "panic" {
		t.Errorf("span saw the panic unwinding past it: Recovery is not inside Tracing")
	}

	if len(mp.hist.sets) != 1 {
		t.Fatalf("duration records = %d", len(mp.hist.sets))
	}
	if v, ok := mp.hist.sets[0].Value("http.response.status_code"); !ok || v.AsInt64() != 500 {
		t.Errorf("metric status = %v (set=%v)", v, mp.hist.sets[0].ToSlice())
	}
	if !trace.SpanContextFromContext(mp.hist.ctxs[0]).IsValid() {
		t.Error("duration record has no span context: Metrics is not inside Tracing")
	}

	if len(be.entries) != 2 {
		t.Fatalf("log entries = %d, want panic entry + access entry: %+v", len(be.entries), be.entries)
	}
	for _, e := range be.entries {
		for _, k := range []string{requestid.LogKey, telemetry.TraceIDKey, telemetry.SpanIDKey} {
			if _, ok := e.attrs[k]; !ok {
				t.Errorf("entry %q lacks %s: %v", e.msg, k, e.attrs)
			}
		}
	}
	var access *entry
	for i := range be.entries {
		if be.entries[i].msg == "/items/7" {
			access = &be.entries[i]
		}
	}
	if access == nil {
		t.Fatalf("no access entry: %+v", be.entries)
	}
	if access.attrs["route"] != "/items/:id" {
		t.Errorf("route = %v", access.attrs["route"])
	}
	if got := access.attrs[requestid.LogKey]; got != rid {
		t.Errorf("access request_id = %v, header = %s", got, rid)
	}
}

func TestRecoveryErrorSurfacesToOuterLayers(t *testing.T) {
	be := &capture{}
	serve(t, []Option{WithLogger(log.NewLogger(be)), WithRecoveryError()}, nil, "/items/7")
	var access *entry
	for i := range be.entries {
		if be.entries[i].msg == "/items/7" {
			access = &be.entries[i]
		}
	}
	if access == nil || access.level != log.LevelError {
		t.Fatalf("access entry = %+v", access)
	}
}

// TestUnmatchedPathCoveredAtEngine pins the engine-level rule: the stack runs
// for a path no route matched, with the low-cardinality route label.
func TestUnmatchedPathCoveredAtEngine(t *testing.T) {
	be := &capture{}
	w := serve(t, []Option{WithLogger(log.NewLogger(be))}, nil, "/nope")
	if w.Code != http.StatusNotFound || w.Header().Get(requestid.DefaultHeader) == "" {
		t.Fatalf("status=%d headers=%v", w.Code, w.Header())
	}
	if len(be.entries) != 1 || be.entries[0].attrs["route"] != "unmatched" {
		t.Fatalf("entries = %+v", be.entries)
	}
}

// TestGroupMiddlewareRunsInsideStack pins the group-level rule: an error from a
// group's auth-style middleware is inside the stack, so it is logged with the
// request id and traced as a span.
func TestGroupMiddlewareRunsInsideStack(t *testing.T) {
	be := &capture{}
	tp := &fakeTracerProvider{}
	errDenied := errors.New("denied")
	app := stdx.New()
	if err := Apply(app, WithLogger(log.NewLogger(be)), WithTracing(telemetry.WithTracerProvider(tp))); err != nil {
		t.Fatal(err)
	}
	g := app.Group("/private", func(next httpx.Handler) httpx.Handler {
		return func(httpx.Context) error { return errDenied }
	})
	g.GET("/x", func(httpx.Context) error { return nil })
	w := httptest.NewRecorder()
	app.(http.Handler).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/private/x", nil))
	if len(be.entries) != 1 || be.entries[0].level != log.LevelError {
		t.Fatalf("entries = %+v", be.entries)
	}
	if _, ok := be.entries[0].attrs[requestid.LogKey]; !ok {
		t.Errorf("entry lacks request id: %v", be.entries[0].attrs)
	}
	if len(tp.spans) != 1 || tp.spans[0].ends != 1 {
		t.Fatalf("spans = %+v", tp.spans)
	}
}

// TestRecoveryWithoutLogger pins the no-logger default: no access log, but a
// panicking handler still gets a 500.
func TestRecoveryWithoutLogger(t *testing.T) {
	if w := serve(t, nil, nil, "/items/7"); w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestApplyRegistersNothingOnError(t *testing.T) {
	mp := &fakeMeterProvider{hist: &fakeHist{}, err: errors.New("no instruments")}
	app := stdx.New()
	if err := Apply(app, WithMetrics(telemetry.WithMeterProvider(mp))); err == nil {
		t.Fatal("want instrument creation error")
	}
	app.Group("").GET("/ok", func(ctx httpx.Context) error { return ctx.NoContent(http.StatusNoContent) })
	w := httptest.NewRecorder()
	app.(http.Handler).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if w.Header().Get(requestid.DefaultHeader) != "" {
		t.Fatal("middleware was registered despite the error")
	}
}
