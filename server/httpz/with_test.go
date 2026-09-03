package httpz

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sphere/httpx"
)

// withFakeContext embeds httpx.Context so it satisfies the whole interface while
// overriding only the methods the wrappers under test actually call.
type withFakeContext struct {
	httpxContext
	status int
	body   any
	text   string
	values map[string]any
}

func (f *withFakeContext) JSON(code int, v any) error {
	f.status = code
	f.body = v
	return nil
}

func (f *withFakeContext) Text(code int, s string) error {
	f.status = code
	f.text = s
	return nil
}

func (f *withFakeContext) Get(key string) (any, bool) {
	v, ok := f.values[key]
	return v, ok
}

// StatusCode implements httpx.ResponseInfo so the wrappers can observe a status
// buffered by the handler.
func (f *withFakeContext) StatusCode() int { return f.status }

// TestWithJsonSuccessEnvelope pins the shape of a successful response: the
// standard envelope with Success set and the value in Data, at 200 when the
// handler did not buffer another status.
func TestWithJsonSuccessEnvelope(t *testing.T) {
	ctx := &withFakeContext{}
	handler := WithJson(func(httpx.Context) (map[string]string, error) {
		return map[string]string{"id": "1"}, nil
	})

	if err := handler(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if ctx.status != http.StatusOK {
		t.Fatalf("status = %d, want %d", ctx.status, http.StatusOK)
	}
	resp, ok := ctx.body.(DataResponse[map[string]string])
	if !ok {
		t.Fatalf("body type = %T, want DataResponse[map[string]string]", ctx.body)
	}
	if !resp.Success {
		t.Error("Success = false, want true")
	}
	if resp.Data["id"] != "1" {
		t.Errorf("Data = %v, want id=1", resp.Data)
	}
}

// TestWithJsonRespectsBufferedStatus pins that a status the handler buffered via
// ResponseInfo (e.g. 201 Created) is kept instead of being flattened to 200, and
// that an out-of-range value is rejected rather than written as an invalid code.
func TestWithJsonRespectsBufferedStatus(t *testing.T) {
	t.Run("buffered 201", func(t *testing.T) {
		ctx := &withFakeContext{}
		handler := WithJson(func(httpx.Context) (string, error) {
			ctx.status = http.StatusCreated
			return "created", nil
		})

		if err := handler(ctx); err != nil {
			t.Fatalf("handler: %v", err)
		}
		if ctx.status != http.StatusCreated {
			t.Fatalf("status = %d, want %d", ctx.status, http.StatusCreated)
		}
	})

	t.Run("out of range falls back to 200", func(t *testing.T) {
		ctx := &withFakeContext{}
		handler := WithJson(func(httpx.Context) (string, error) {
			ctx.status = 42
			return "created", nil
		})

		if err := handler(ctx); err != nil {
			t.Fatalf("handler: %v", err)
		}
		if ctx.status != http.StatusOK {
			t.Fatalf("status = %d, want %d", ctx.status, http.StatusOK)
		}
	})
}

// TestWithJsonErrorPath pins that a handler error flows through the standard
// error response path instead of being wrapped in a 200 envelope.
func TestWithJsonErrorPath(t *testing.T) {
	ctx := &withFakeContext{}
	handler := WithJson(func(httpx.Context) (string, error) {
		return "", httpx.BadRequestError(errors.New("raw detail"), "please provide a valid id")
	})

	if err := handler(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if ctx.status != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", ctx.status, http.StatusBadRequest)
	}
	resp, ok := ctx.body.(ErrorResponse)
	if !ok {
		t.Fatalf("body type = %T, want ErrorResponse", ctx.body)
	}
	if resp.Message != "please provide a valid id" {
		t.Errorf("Message = %q, want the classified message", resp.Message)
	}
}

// TestWithText pins both directions of the plain-text wrapper.
func TestWithText(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctx := &withFakeContext{}
		handler := WithText(func(httpx.Context) (string, error) {
			return "hello", nil
		})

		if err := handler(ctx); err != nil {
			t.Fatalf("handler: %v", err)
		}
		if ctx.status != http.StatusOK || ctx.text != "hello" {
			t.Fatalf("got (%d, %q), want (200, hello)", ctx.status, ctx.text)
		}
	})

	t.Run("error", func(t *testing.T) {
		ctx := &withFakeContext{}
		handler := WithText(func(httpx.Context) (string, error) {
			return "", errors.New("unclassified failure")
		})

		if err := handler(ctx); err != nil {
			t.Fatalf("handler: %v", err)
		}
		if ctx.status != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", ctx.status, http.StatusInternalServerError)
		}
		if _, ok := ctx.body.(ErrorResponse); !ok {
			t.Fatalf("body type = %T, want ErrorResponse", ctx.body)
		}
	})
}

// TestWithRecoverTurnsPanicsInto500s pins the wrapper's core promise: a handler
// panic must not take the request goroutine (or process) down, and the panic's
// text must not leak to the client.
func TestWithRecoverTurnsPanicsInto500s(t *testing.T) {
	prev := DebugMode()
	SetDebugMode(false)
	defer SetDebugMode(prev)

	ctx := &withFakeContext{}
	handler := WithRecover("boom", func(httpx.Context) error {
		panic("sensitive internal state: /var/secrets/key.pem")
	})

	if err := handler(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if ctx.status != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", ctx.status, http.StatusInternalServerError)
	}
	resp, ok := ctx.body.(ErrorResponse)
	if !ok {
		t.Fatalf("body type = %T, want ErrorResponse", ctx.body)
	}
	if resp.Error != "" {
		t.Fatalf("panic text leaked through Error: %q", resp.Error)
	}
	if resp.Message == "" || resp.Message == "sensitive internal state: /var/secrets/key.pem" {
		t.Fatalf("Message = %q, want the generic status text", resp.Message)
	}
}

// TestWithRecoverKeepsClassifiedErrorPath pins that recover does not interfere
// with the ordinary error path: a handler returning an error still produces the
// classified response.
func TestWithRecoverKeepsClassifiedErrorPath(t *testing.T) {
	ctx := &withFakeContext{}
	handler := WithRecover("boom", func(httpx.Context) error {
		return httpx.UnauthorizedError(errors.New("token expired"))
	})

	if err := handler(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if ctx.status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", ctx.status, http.StatusUnauthorized)
	}
}

// TestWithRecoverRepanicsAbortHandler pins that a handler abandoning the request
// via http.ErrAbortHandler is left to net/http, which silently drops the
// connection. Turning it into a 500 write on a dead connection is the failure
// mode this guard prevents.
func TestWithRecoverRepanicsAbortHandler(t *testing.T) {
	handler := WithRecover("boom", func(httpx.Context) error {
		panic(http.ErrAbortHandler)
	})

	defer func() {
		if got := recover(); got != http.ErrAbortHandler {
			t.Fatalf("recovered %v, want http.ErrAbortHandler to propagate", got)
		}
	}()
	_ = handler(&withFakeContext{})
	t.Fatal("http.ErrAbortHandler must not be swallowed")
}

// TestValue pins the typed context lookup: present and correct type yields the
// value, anything else yields the zero value with ok=false.
func TestValue(t *testing.T) {
	ctx := &withFakeContext{values: map[string]any{
		"count": 3,
		"name":  "alice",
	}}

	if got, ok := Value[int](ctx, "count"); !ok || got != 3 {
		t.Fatalf("Value[int] = (%d, %v), want (3, true)", got, ok)
	}
	if got, ok := Value[string](ctx, "name"); !ok || got != "alice" {
		t.Fatalf("Value[string] = (%q, %v), want (alice, true)", got, ok)
	}
	if got, ok := Value[string](ctx, "count"); ok || got != "" {
		t.Fatalf("cross-type read = (%q, %v), want (\"\", false)", got, ok)
	}
	if got, ok := Value[int](ctx, "missing"); ok || got != 0 {
		t.Fatalf("missing key = (%d, %v), want (0, false)", got, ok)
	}
}

type stressFakeContext struct {
	httpxContext
	mu     sync.Mutex
	status int
	body   any
	text   string
}

func (f *stressFakeContext) JSON(code int, v any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status = code
	f.body = v
	return nil
}

func (f *stressFakeContext) Text(code int, s string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status = code
	f.text = s
	return nil
}

type customCodeMsgError struct {
	code int32
	msg  string
	err  error
}

func (e *customCodeMsgError) Error() string { return e.err.Error() }

func (e *customCodeMsgError) GetCode() int32 { return e.code }

func (e *customCodeMsgError) GetMessage() string { return e.msg }

// TestStressPanicStorm executes thousands of concurrent panics with diverse types
// while concurrently toggling debug mode and default error parser.
func TestStressPanicStorm(t *testing.T) {
	prevDebug := DebugMode()
	t.Cleanup(func() {
		SetDebugMode(prevDebug)
		SetDefaultErrorParser(httpx.ParseError)
	})

	const numGoroutines = 50
	const iterations = 200

	var panicCount atomic.Int64
	var abortHandlerCount atomic.Int64
	var totalRuns atomic.Int64

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Flapper goroutine for config
	wg.Go(func() {
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
				switch i % 3 {
				case 0:
					SetDefaultErrorParser(nil) // should be ignored safely
				case 1:
					SetDefaultErrorParser(httpx.ParseError)
				default:
					SetDefaultErrorParser(func(err error) (int32, int32, string) {
						return 1001, http.StatusUnprocessableEntity, "custom entity error"
					})
				}
				i++
				time.Sleep(100 * time.Microsecond)
			}
		}
	})

	// Worker goroutines
	for g := range numGoroutines {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for iter := range iterations {
				totalRuns.Add(1)
				panicKind := (workerID + iter) % 7

				ctx := &stressFakeContext{}
				h := WithRecover("stress panic handler", func(c httpx.Context) error {
					switch panicKind {
					case 0:
						panic("secret_db_password_12345")
					case 1:
						panic(errors.New("sql: syntax error at or near WHERE /var/lib/postgresql/data"))
					case 2:
						panic(9999)
					case 3:
						panic(struct{ Secret string }{Secret: "top_secret_token"})
					case 4:
						panic(http.ErrAbortHandler)
					case 5:
						return errors.New("normal error return")
					case 6:
						return &customCodeMsgError{
							code: 4001,
							msg:  "friendly user message",
							err:  errors.New("internal sensitive detail in struct"),
						}
					default:
						return nil
					}
				})

				if panicKind == 4 {
					// http.ErrAbortHandler MUST re-panic
					func() {
						defer func() {
							r := recover()
							if r == http.ErrAbortHandler {
								abortHandlerCount.Add(1)
							} else {
								t.Errorf("expected http.ErrAbortHandler, got %v", r)
							}
						}()
						_ = h(ctx)
						t.Errorf("expected panic for ErrAbortHandler, got normal exit")
					}()
				} else {
					// Must not panic
					err := h(ctx)
					if err != nil {
						t.Errorf("expected WithRecover to return nil error, got %v", err)
					}
					panicCount.Add(1)
				}
			}
		}(g)
	}

	// Let workers run
	go func() {
		time.Sleep(500 * time.Millisecond)
		close(stop)
	}()

	wg.Wait()

	t.Logf("Stress completed: total runs=%d, recovered panics=%d, abortHandler panics=%d",
		totalRuns.Load(), panicCount.Load(), abortHandlerCount.Load())
}

// TestStressNoLeakageUnderNonDebug verifies strict data leakage prevention under diverse errors.
func TestStressNoLeakageUnderNonDebug(t *testing.T) {
	prevDebug := DebugMode()
	SetDebugMode(false)
	SetDefaultErrorParser(httpx.ParseError)
	t.Cleanup(func() {
		SetDebugMode(prevDebug)
		SetDefaultErrorParser(httpx.ParseError)
	})

	leakPatterns := []string{
		"password",
		"10.0.3.14",
		"SELECT * FROM",
		"/var/secrets/key.pem",
		"internal_token_xyz",
		"fatal panic info",
	}

	testCases := []struct {
		name        string
		makeHandler func() httpx.Handler
	}{
		{
			name: "panic string with secrets",
			makeHandler: func() httpx.Handler {
				return WithRecover("panic", func(c httpx.Context) error {
					panic("connection failed: password=super_secret host=10.0.3.14:5432")
				})
			},
		},
		{
			name: "panic error with stack / paths",
			makeHandler: func() httpx.Handler {
				return WithRecover("panic", func(c httpx.Context) error {
					panic(errors.New("open /var/secrets/key.pem: permission denied"))
				})
			},
		},
		{
			name: "unclassified sql error return",
			makeHandler: func() httpx.Handler {
				return WithRecover("err", func(c httpx.Context) error {
					return errors.New("pq: syntax error at SELECT * FROM users WHERE password = 1")
				})
			},
		},
		{
			name: "wrapped unclassified error",
			makeHandler: func() httpx.Handler {
				return WithRecover("err", func(c httpx.Context) error {
					return fmt.Errorf("service layer: %w", errors.New("failed with internal_token_xyz"))
				})
			},
		},
		{
			name: "WithJson returning unclassified error",
			makeHandler: func() httpx.Handler {
				return WithJson(func(c httpx.Context) (any, error) {
					return nil, errors.New("fatal panic info in json handler")
				})
			},
		},
		{
			name: "WithText returning unclassified error",
			makeHandler: func() httpx.Handler {
				return WithText(func(c httpx.Context) (string, error) {
					return "", errors.New("fatal panic info in text handler")
				})
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &stressFakeContext{}
			h := tc.makeHandler()
			_ = h(ctx)

			resp, ok := ctx.body.(ErrorResponse)
			if !ok {
				t.Fatalf("expected ErrorResponse, got %T", ctx.body)
			}

			// Under non-debug, resp.Error MUST be empty
			if resp.Error != "" {
				t.Errorf("Error field leaked raw content: %q", resp.Error)
			}

			// Check Message field for any sensitive leakage
			for _, pattern := range leakPatterns {
				if strings.Contains(resp.Message, pattern) {
					t.Errorf("Message field leaked sensitive pattern %q: %q", pattern, resp.Message)
				}
			}

			// Code should be 0 for unclassified errors
			if resp.Code != 0 {
				t.Errorf("expected Code=0 for unclassified, got %d", resp.Code)
			}
		})
	}
}
