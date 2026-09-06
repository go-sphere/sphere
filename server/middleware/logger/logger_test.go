package logger

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/log"
)

type httpxContext = httpx.Context

// fakeContext embeds httpx.Context so it satisfies the full interface while
// only overriding the methods Log and RecoveryLog actually call.
type fakeContext struct {
	httpxContext
	ctx     context.Context
	method  string
	path    string
	query   string
	ip      string
	headers map[string]string
	status  int
	next    func() error
}

func (f *fakeContext) Context() context.Context {
	if f.ctx == nil {
		return context.Background()
	}
	return f.ctx
}

func (f *fakeContext) Method() string   { return f.method }
func (f *fakeContext) Path() string     { return f.path }
func (f *fakeContext) RawQuery() string { return f.query }
func (f *fakeContext) ClientIP() string { return f.ip }

func (f *fakeContext) Header(key string) string {
	if f.headers == nil {
		return ""
	}
	return f.headers[key]
}

func (f *fakeContext) StatusCode() int { return f.status }

func (f *fakeContext) Status(code int) { f.status = code }

func (f *fakeContext) NoContent(code int) error {
	f.status = code
	return nil
}

func (f *fakeContext) Next() error {
	if f.next != nil {
		return f.next()
	}
	return nil
}

type entry struct {
	level log.Level
	msg   string
	attrs []log.Attr
}

// recordingLogger is a log.BaseLogger that records Info/Error (and the other
// levels so the interface is satisfied).
type recordingLogger struct {
	entries []entry
}

func (r *recordingLogger) Debug(msg string, attrs ...log.Attr) {
	r.entries = append(r.entries, entry{level: log.LevelDebug, msg: msg, attrs: slices.Clone(attrs)})
}

func (r *recordingLogger) Info(msg string, attrs ...log.Attr) {
	r.entries = append(r.entries, entry{level: log.LevelInfo, msg: msg, attrs: slices.Clone(attrs)})
}

func (r *recordingLogger) Warn(msg string, attrs ...log.Attr) {
	r.entries = append(r.entries, entry{level: log.LevelWarn, msg: msg, attrs: slices.Clone(attrs)})
}

func (r *recordingLogger) Error(msg string, attrs ...log.Attr) {
	r.entries = append(r.entries, entry{level: log.LevelError, msg: msg, attrs: slices.Clone(attrs)})
}

var _ log.BaseLogger = (*recordingLogger)(nil)

func requireAttr(t *testing.T, attrs []log.Attr, key string) slog.Value {
	t.Helper()
	for _, a := range attrs {
		if a.Key == key {
			return a.Value
		}
	}
	t.Fatalf("missing attr %q", key)
	return slog.Value{}
}

func hasAttr(attrs []log.Attr, key string) bool {
	return slices.ContainsFunc(attrs, func(a log.Attr) bool {
		return a.Key == key
	})
}

func accessRequest() *fakeContext {
	return &fakeContext{
		method: http.MethodGet,
		path:   "/users",
		query:  "q=alice",
		ip:     "203.0.113.10",
		headers: map[string]string{
			"User-Agent": "logger-test/1.0",
		},
		status: http.StatusOK,
	}
}

func assertAccessAttrs(t *testing.T, attrs []log.Attr, req *fakeContext, status int) {
	t.Helper()
	if got := requireAttr(t, attrs, "method").String(); got != req.method {
		t.Errorf("method = %q, want %q", got, req.method)
	}
	if got := requireAttr(t, attrs, "path").String(); got != req.path {
		t.Errorf("path = %q, want %q", got, req.path)
	}
	if got := requireAttr(t, attrs, "query").String(); got != req.query {
		t.Errorf("query = %q, want %q", got, req.query)
	}
	if got := requireAttr(t, attrs, "ip").String(); got != req.ip {
		t.Errorf("ip = %q, want %q", got, req.ip)
	}
	if got := requireAttr(t, attrs, "user-agent").String(); got != req.headers["User-Agent"] {
		t.Errorf("user-agent = %q, want %q", got, req.headers["User-Agent"])
	}
	statusVal := requireAttr(t, attrs, "status")
	if statusVal.Kind() != slog.KindInt64 {
		t.Errorf("status kind = %v, want Int64", statusVal.Kind())
	}
	if got := statusVal.Int64(); got != int64(status) {
		t.Errorf("status = %d, want %d", got, status)
	}
	latency := requireAttr(t, attrs, "latency")
	if latency.Kind() != slog.KindDuration {
		t.Errorf("latency kind = %v, want Duration", latency.Kind())
	}
	if d := latency.Duration(); d < 0 {
		t.Errorf("latency = %v, want non-negative", d)
	}
}

// TestLogSuccess pins that a successful chain produces exactly one Info
// access entry with the request fields the middleware is documented to log.
func TestLogSuccess(t *testing.T) {
	t.Parallel()

	rec := &recordingLogger{}
	req := accessRequest()
	req.ctx = t.Context()
	req.next = func() error {
		req.status = http.StatusOK
		return nil
	}

	if err := Log(rec)(req); err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(rec.entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(rec.entries))
	}
	got := rec.entries[0]
	if got.level != log.LevelInfo {
		t.Errorf("level = %v, want LevelInfo", got.level)
	}
	assertAccessAttrs(t, got.attrs, req, http.StatusOK)
	if hasAttr(got.attrs, "error") {
		t.Error("success path must not include an error attr")
	}
}

// TestLogChainError pins that Next returning an error is logged with Error
// with the same request fields plus the chain error, and that the error is
// still returned to the caller.
func TestLogChainError(t *testing.T) {
	t.Parallel()

	chainErr := errors.New("handler failed")
	rec := &recordingLogger{}
	req := accessRequest()
	req.ctx = t.Context()
	req.next = func() error {
		req.status = http.StatusBadRequest
		return chainErr
	}

	err := Log(rec)(req)
	if !errors.Is(err, chainErr) {
		t.Fatalf("Log() error = %v, want %v", err, chainErr)
	}
	if len(rec.entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(rec.entries))
	}
	got := rec.entries[0]
	if got.level != log.LevelError {
		t.Errorf("level = %v, want LevelError", got.level)
	}
	assertAccessAttrs(t, got.attrs, req, http.StatusBadRequest)
	errVal := requireAttr(t, got.attrs, "error").Any()
	gotErr, ok := errVal.(error)
	if !ok {
		t.Fatalf("error attr type = %T, want error", errVal)
	}
	if !errors.Is(gotErr, chainErr) {
		t.Errorf("error attr = %v, want %v", gotErr, chainErr)
	}
}

// TestRecoveryLogPanic pins that a panic from Next is recovered, logged with
// Error with the panic value, and finished as HTTP 500.
func TestRecoveryLogPanic(t *testing.T) {
	t.Parallel()

	const panicValue = "boom from handler"
	rec := &recordingLogger{}
	req := accessRequest()
	req.ctx = t.Context()
	req.next = func() error {
		panic(panicValue)
	}

	if err := RecoveryLog(rec, false)(req); err != nil {
		t.Fatalf("RecoveryLog returned %v, want nil after recover", err)
	}
	if req.StatusCode() != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", req.StatusCode())
	}
	if len(rec.entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(rec.entries))
	}
	got := rec.entries[0]
	if got.level != log.LevelError {
		t.Errorf("level = %v, want LevelError", got.level)
	}
	if got := requireAttr(t, got.attrs, "error").Any(); got != panicValue {
		t.Errorf("error attr = %#v, want %#v", got, panicValue)
	}
}

// TestRecoveryLogStackOption pins that a stack attr is present only when the
// stack option is enabled.
func TestRecoveryLogStackOption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		stack bool
	}{
		{name: "stack on", stack: true},
		{name: "stack off", stack: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := &recordingLogger{}
			req := accessRequest()
			req.ctx = t.Context()
			req.next = func() error {
				panic("stack-option")
			}

			if err := RecoveryLog(rec, tt.stack)(req); err != nil {
				t.Fatalf("RecoveryLog: %v", err)
			}
			if len(rec.entries) != 1 {
				t.Fatalf("entries = %d, want 1", len(rec.entries))
			}
			got := rec.entries[0]
			if got.level != log.LevelError {
				t.Errorf("level = %v, want LevelError", got.level)
			}
			if requireAttr(t, got.attrs, "error").Any() != "stack-option" {
				t.Errorf("error attr = %#v, want %#v", requireAttr(t, got.attrs, "error").Any(), "stack-option")
			}
			present := hasAttr(got.attrs, "stack")
			if present != tt.stack {
				t.Fatalf("stack attr present = %v, want %v", present, tt.stack)
			}
			if tt.stack {
				stack := requireAttr(t, got.attrs, "stack").String()
				if !strings.Contains(stack, "goroutine") {
					t.Errorf("stack attr %q does not look like a stack trace", stack)
				}
			}
		})
	}
}
