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
	"github.com/go-sphere/httpx/httpxmock"
	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/server/httpz"
)

// The request accessRequest stands for, stated once so the attr assertions do
// not read their expectations back off the context under test.
const (
	accessMethod    = http.MethodGet
	accessPath      = "/users"
	accessQuery     = "q=alice"
	accessIP        = "203.0.113.10"
	accessUserAgent = "logger-test/1.0"
)

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

func accessRequest(ctx context.Context) *httpxmock.Context {
	return httpxmock.NewRequest(accessMethod, accessPath+"?"+accessQuery, nil,
		httpxmock.WithHeader("User-Agent", accessUserAgent),
		httpxmock.WithClientIP(accessIP),
		httpxmock.WithContext(ctx),
	)
}

func assertAccessAttrs(t *testing.T, attrs []log.Attr, status int) {
	t.Helper()
	if got := requireAttr(t, attrs, "method").String(); got != accessMethod {
		t.Errorf("method = %q, want %q", got, accessMethod)
	}
	if got := requireAttr(t, attrs, "path").String(); got != accessPath {
		t.Errorf("path = %q, want %q", got, accessPath)
	}
	if got := requireAttr(t, attrs, "query").String(); got != accessQuery {
		t.Errorf("query = %q, want %q", got, accessQuery)
	}
	if got := requireAttr(t, attrs, "ip").String(); got != accessIP {
		t.Errorf("ip = %q, want %q", got, accessIP)
	}
	if got := requireAttr(t, attrs, "user-agent").String(); got != accessUserAgent {
		t.Errorf("user-agent = %q, want %q", got, accessUserAgent)
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
	ctx := accessRequest(t.Context())
	next := func(c httpx.Context) error {
		c.Status(http.StatusOK)
		return nil
	}

	if err := httpxmock.Run(ctx, next, Log(rec)); err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(rec.entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(rec.entries))
	}
	got := rec.entries[0]
	if got.level != log.LevelInfo {
		t.Errorf("level = %v, want LevelInfo", got.level)
	}
	assertAccessAttrs(t, got.attrs, http.StatusOK)
	if hasAttr(got.attrs, "error") {
		t.Error("success path must not include an error attr")
	}
}

// TestLogChainError pins that an error returned by the rest of the chain is
// logged with Error with the same request fields plus the chain error, and
// that the error is still returned to the caller.
func TestLogChainError(t *testing.T) {
	t.Parallel()

	chainErr := errors.New("handler failed")
	rec := &recordingLogger{}
	ctx := accessRequest(t.Context())
	next := func(c httpx.Context) error {
		c.Status(http.StatusBadRequest)
		return chainErr
	}

	err := httpxmock.Run(ctx, next, Log(rec))
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
	assertAccessAttrs(t, got.attrs, http.StatusBadRequest)
	errVal := requireAttr(t, got.attrs, "error").Any()
	gotErr, ok := errVal.(error)
	if !ok {
		t.Fatalf("error attr type = %T, want error", errVal)
	}
	if !errors.Is(gotErr, chainErr) {
		t.Errorf("error attr = %v, want %v", gotErr, chainErr)
	}
}

// TestRecoveryLogPanic pins that a panic from the rest of the chain is
// recovered, logged with Error with the panic value, and finished as HTTP 500.
func TestRecoveryLogPanic(t *testing.T) {
	t.Parallel()

	const panicValue = "boom from handler"
	rec := &recordingLogger{}
	ctx := accessRequest(t.Context())
	next := func(httpx.Context) error {
		panic(panicValue)
	}

	if err := httpxmock.Run(ctx, next, RecoveryLog(rec, false)); err != nil {
		t.Fatalf("RecoveryLog returned %v, want nil after recover", err)
	}
	if ctx.StatusCode() != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", ctx.StatusCode())
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
			ctx := accessRequest(t.Context())
			next := func(httpx.Context) error {
				panic("stack-option")
			}

			if err := httpxmock.Run(ctx, next, RecoveryLog(rec, tt.stack)); err != nil {
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

// TestRecoveryLogRepanicsErrAbortHandler pins that RecoveryLog, like
// httpz.WithRecover, lets http.ErrAbortHandler propagate so net/http can drop
// the connection, and does not log it or write a 500.
func TestRecoveryLogRepanicsErrAbortHandler(t *testing.T) {
	t.Parallel()

	rec := &recordingLogger{}
	ctx := accessRequest(t.Context())
	next := func(httpx.Context) error {
		panic(http.ErrAbortHandler)
	}

	defer func() {
		if got := recover(); got != http.ErrAbortHandler {
			t.Fatalf("recovered %v, want http.ErrAbortHandler", got)
		}
		if len(rec.entries) != 0 {
			t.Fatalf("entries = %d, want 0 for an aborted handler", len(rec.entries))
		}
		if ctx.StatusCode() == http.StatusInternalServerError {
			t.Fatal("an aborted handler must not be finished as 500")
		}
	}()
	_ = httpxmock.Run(ctx, next, RecoveryLog(rec, false))
	t.Fatal("RecoveryLog swallowed http.ErrAbortHandler")
}

// TestLogUsesInstalledErrorParser pins that the logged status for a chain
// error that wrote no status comes from the parser installed with
// httpz.SetDefaultErrorParser, i.e. the status the client is sent. Not
// parallel: it swaps the process-wide parser.
func TestLogUsesInstalledErrorParser(t *testing.T) {
	t.Cleanup(func() { httpz.SetDefaultErrorParser(httpz.ParseError) })
	httpz.SetDefaultErrorParser(func(error) (int32, int32, string) {
		return 0, http.StatusTeapot, "teapot"
	})

	rec := &recordingLogger{}
	ctx := accessRequest(t.Context())
	chainErr := errors.New("unclassified")
	next := func(httpx.Context) error { return chainErr }

	if err := httpxmock.Run(ctx, next, Log(rec)); !errors.Is(err, chainErr) {
		t.Fatalf("Log() error = %v, want %v", err, chainErr)
	}
	if len(rec.entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(rec.entries))
	}
	if got := requireAttr(t, rec.entries[0].attrs, "status").Int64(); got != http.StatusTeapot {
		t.Fatalf("logged status = %d, want %d from the installed parser", got, http.StatusTeapot)
	}
}
