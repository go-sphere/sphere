package telemetry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/stdx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/embedded"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// The fakes need only the API modules; no SDK is pulled into go.mod.

type fakeSpan struct {
	tracenoop.Span
	mu     sync.Mutex
	name   string
	kind   trace.SpanKind
	links  []trace.Link
	parent trace.SpanContext
	attrs  map[string]attribute.Value
	code   codes.Code
	desc   string
	errs   []error
	ends   int
	sc     trace.SpanContext
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
func (s *fakeSpan) SetStatus(c codes.Code, d string) {
	s.mu.Lock()
	s.code, s.desc = c, d
	s.mu.Unlock()
}
func (s *fakeSpan) RecordError(err error, _ ...trace.EventOption) {
	s.mu.Lock()
	s.errs = append(s.errs, err)
	s.mu.Unlock()
}
func (s *fakeSpan) End(...trace.SpanEndOption) { s.mu.Lock(); s.ends++; s.mu.Unlock() }

type fakeTracer struct {
	embedded.Tracer
	tp *fakeTracerProvider
}

func (t *fakeTracer) Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	cfg := trace.NewSpanStartConfig(opts...)
	t.tp.mu.Lock()
	defer t.tp.mu.Unlock()
	n := len(t.tp.spans) + 1
	s := &fakeSpan{
		name: name, kind: cfg.SpanKind(), links: cfg.Links(), attrs: map[string]attribute.Value{},
		parent: trace.SpanContextFromContext(ctx),
	}
	if cfg.NewRoot() {
		s.parent = trace.SpanContext{}
	}
	for _, a := range cfg.Attributes() {
		s.attrs[string(a.Key)] = a.Value
	}
	tid := trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, byte(n)}
	if s.parent.IsValid() {
		tid = s.parent.TraceID()
	}
	s.sc = trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: tid, SpanID: trace.SpanID{0, 0, 0, 0, 0, 0, 0, byte(n)}, TraceFlags: trace.FlagsSampled,
	})
	t.tp.spans = append(t.tp.spans, s)
	return trace.ContextWithSpan(ctx, s), s
}

type fakeTracerProvider struct {
	embedded.TracerProvider
	mu    sync.Mutex
	spans []*fakeSpan
}

func (p *fakeTracerProvider) Tracer(string, ...trace.TracerOption) trace.Tracer {
	return &fakeTracer{tp: p}
}

type fakeHist struct {
	metricnoop.Float64Histogram
	mu   sync.Mutex
	sets []attribute.Set
	vals []float64
}

func (h *fakeHist) Enabled(context.Context) bool { return true }
func (h *fakeHist) Record(_ context.Context, v float64, opts ...metric.RecordOption) {
	cfg := metric.NewRecordConfig(opts)
	h.mu.Lock()
	h.sets = append(h.sets, cfg.Attributes())
	h.vals = append(h.vals, v)
	h.mu.Unlock()
}

type fakeCounter struct {
	metricnoop.Int64UpDownCounter
	mu   sync.Mutex
	sets []attribute.Set
	adds []int64
}

func (c *fakeCounter) Enabled(context.Context) bool { return true }
func (c *fakeCounter) Add(_ context.Context, v int64, opts ...metric.AddOption) {
	cfg := metric.NewAddConfig(opts)
	c.mu.Lock()
	c.sets = append(c.sets, cfg.Attributes())
	c.adds = append(c.adds, v)
	c.mu.Unlock()
}
func (c *fakeCounter) sum() (n int64) {
	for _, v := range c.adds {
		n += v
	}
	return n
}

type fakeMeterProvider struct {
	metricnoop.MeterProvider
	hist *fakeHist
	ctr  *fakeCounter
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
func (m *fakeMeter) Int64UpDownCounter(string, ...metric.Int64UpDownCounterOption) (metric.Int64UpDownCounter, error) {
	return m.p.ctr, m.p.err
}

func newMP() *fakeMeterProvider {
	return &fakeMeterProvider{hist: &fakeHist{}, ctr: &fakeCounter{}}
}

func setOf(s attribute.Set) map[string]attribute.Value {
	m := map[string]attribute.Value{}
	for _, kv := range s.ToSlice() {
		m[string(kv.Key)] = kv.Value
	}
	return m
}

// serve builds a stdx engine with mws and routes, then serves one request.
func serve(t *testing.T, mws []httpx.Middleware, setup func(httpx.Router), method, target string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	app := stdx.New()
	app.Use(mws...)
	setup(app.Group(""))
	req := httptest.NewRequest(method, target, nil)
	for k, v := range hdr {
		req.Header[k] = []string{v}
	}
	w := httptest.NewRecorder()
	app.(http.Handler).ServeHTTP(w, req)
	return w
}

func mustMetrics(t *testing.T, opts ...Option) httpx.Middleware {
	t.Helper()
	m, err := NewMetrics(opts...)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

var errBoom = errors.New("boom")
