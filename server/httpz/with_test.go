package httpz

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/httpxmock"
)

// lastWrite returns the last body-writing Responder call the wrappers made, so
// a test can tell "answered with NoContent" from "happened to write a 204".
func lastWrite(t *testing.T, ctx *httpxmock.Context) httpxmock.ResponseWrite {
	t.Helper()
	writes := ctx.Writes()
	if len(writes) == 0 {
		t.Fatal("no response was written")
	}
	return writes[len(writes)-1]
}

// jsonBody returns the value the wrappers handed to JSON, typed.
func jsonBody[T any](t *testing.T, ctx *httpxmock.Context) T {
	t.Helper()
	v, ok := ctx.LastJSON()
	if !ok {
		t.Fatal("no JSON response was written")
	}
	typed, ok := v.(T)
	if !ok {
		var zero T
		t.Fatalf("body type = %T, want %T", v, zero)
	}
	return typed
}

// TestWithJsonSuccessEnvelope pins the shape of a successful response: the
// standard envelope with Success set and the value in Data, at 200 when the
// handler did not buffer another status.
func TestWithJsonSuccessEnvelope(t *testing.T) {
	ctx := httpxmock.New(nil)
	handler := WithJson(func(httpx.Context) (map[string]string, error) {
		return map[string]string{"id": "1"}, nil
	})

	if err := handler(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if ctx.StatusCode() != http.StatusOK {
		t.Fatalf("status = %d, want %d", ctx.StatusCode(), http.StatusOK)
	}
	resp := jsonBody[DataResponse[map[string]string]](t, ctx)
	if !resp.Success {
		t.Error("Success = false, want true")
	}
	if resp.Data["id"] != "1" {
		t.Errorf("Data = %v, want id=1", resp.Data)
	}
}

// TestWithJsonRespectsBufferedStatus pins that a status the handler buffered via
// ctx.Status (e.g. 201 Created) is kept instead of being flattened to 200, and
// that an out-of-range value is rejected rather than written as an invalid code.
func TestWithJsonRespectsBufferedStatus(t *testing.T) {
	t.Run("buffered 201", func(t *testing.T) {
		ctx := httpxmock.New(nil)
		handler := WithJson(func(httpx.Context) (string, error) {
			ctx.Status(http.StatusCreated)
			return "created", nil
		})

		if err := handler(ctx); err != nil {
			t.Fatalf("handler: %v", err)
		}
		if ctx.StatusCode() != http.StatusCreated {
			t.Fatalf("status = %d, want %d", ctx.StatusCode(), http.StatusCreated)
		}
	})

	t.Run("out of range falls back to 200", func(t *testing.T) {
		ctx := httpxmock.New(nil)
		handler := WithJson(func(httpx.Context) (string, error) {
			ctx.Status(42)
			return "created", nil
		})

		if err := handler(ctx); err != nil {
			t.Fatalf("handler: %v", err)
		}
		if ctx.StatusCode() != http.StatusOK {
			t.Fatalf("status = %d, want %d", ctx.StatusCode(), http.StatusOK)
		}
	})

	t.Run("buffered 204 writes NoContent, not a JSON body", func(t *testing.T) {
		ctx := httpxmock.New(nil)
		handler := WithJson(func(httpx.Context) (string, error) {
			ctx.Status(http.StatusNoContent)
			return "gone", nil
		})

		if err := handler(ctx); err != nil {
			t.Fatalf("handler: %v", err)
		}
		if w := lastWrite(t, ctx); w.Kind != httpxmock.KindNoContent || w.Code != http.StatusNoContent {
			t.Fatalf("last write = %s %d, want NoContent %d", w.Kind, w.Code, http.StatusNoContent)
		}
		if v, ok := ctx.LastJSON(); ok {
			t.Fatalf("JSON body written for 204: %v", v)
		}
	})

	t.Run("buffered 304 writes NoContent, not a JSON body", func(t *testing.T) {
		ctx := httpxmock.New(nil)
		handler := WithJson(func(httpx.Context) (string, error) {
			ctx.Status(http.StatusNotModified)
			return "cached", nil
		})

		if err := handler(ctx); err != nil {
			t.Fatalf("handler: %v", err)
		}
		if w := lastWrite(t, ctx); w.Kind != httpxmock.KindNoContent || w.Code != http.StatusNotModified {
			t.Fatalf("last write = %s %d, want NoContent %d", w.Kind, w.Code, http.StatusNotModified)
		}
		if v, ok := ctx.LastJSON(); ok {
			t.Fatalf("JSON body written for 304: %v", v)
		}
	})
}

// TestWithJsonErrorPath pins that a handler error flows through the standard
// error response path instead of being wrapped in a 200 envelope.
func TestWithJsonErrorPath(t *testing.T) {
	ctx := httpxmock.New(nil)
	handler := WithJson(func(httpx.Context) (string, error) {
		return "", httpx.BadRequestError(errors.New("raw detail"), "please provide a valid id")
	})

	if err := handler(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if ctx.StatusCode() != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", ctx.StatusCode(), http.StatusBadRequest)
	}
	resp := jsonBody[ErrorResponse](t, ctx)
	if resp.Message != "please provide a valid id" {
		t.Errorf("Message = %q, want the classified message", resp.Message)
	}
}

// TestWithText pins both directions of the plain-text wrapper.
func TestWithText(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctx := httpxmock.New(nil)
		handler := WithText(func(httpx.Context) (string, error) {
			return "hello", nil
		})

		if err := handler(ctx); err != nil {
			t.Fatalf("handler: %v", err)
		}
		if ctx.StatusCode() != http.StatusOK || ctx.BodyString() != "hello" {
			t.Fatalf("got (%d, %q), want (200, hello)", ctx.StatusCode(), ctx.BodyString())
		}
	})

	t.Run("error", func(t *testing.T) {
		ctx := httpxmock.New(nil)
		handler := WithText(func(httpx.Context) (string, error) {
			return "", errors.New("unclassified failure")
		})

		if err := handler(ctx); err != nil {
			t.Fatalf("handler: %v", err)
		}
		if ctx.StatusCode() != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", ctx.StatusCode(), http.StatusInternalServerError)
		}
		_ = jsonBody[ErrorResponse](t, ctx)
	})
}

// TestWithRecoverTurnsPanicsInto500s pins the wrapper's core promise: a handler
// panic must not take the request goroutine (or process) down, and the panic's
// text must not leak to the client.
func TestWithRecoverTurnsPanicsInto500s(t *testing.T) {
	prev := DebugMode()
	SetDebugMode(false)
	defer SetDebugMode(prev)

	ctx := httpxmock.New(nil)
	handler := WithRecover("boom", func(httpx.Context) error {
		panic("sensitive internal state: /var/secrets/key.pem")
	})

	if err := handler(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if ctx.StatusCode() != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", ctx.StatusCode(), http.StatusInternalServerError)
	}
	resp := jsonBody[ErrorResponse](t, ctx)
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
	ctx := httpxmock.New(nil)
	handler := WithRecover("boom", func(httpx.Context) error {
		return httpx.UnauthorizedError(errors.New("token expired"))
	})

	if err := handler(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if ctx.StatusCode() != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", ctx.StatusCode(), http.StatusUnauthorized)
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
	_ = handler(httpxmock.New(nil))
	t.Fatal("http.ErrAbortHandler must not be swallowed")
}

// TestValue pins the typed context lookup: present and correct type yields the
// value, anything else yields the zero value with ok=false.
func TestValue(t *testing.T) {
	ctx := httpxmock.New(nil,
		httpxmock.WithState("count", 3),
		httpxmock.WithState("name", "alice"),
	)

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
			ctx := httpxmock.New(nil)
			h := tc.makeHandler()
			_ = h(ctx)

			resp := jsonBody[ErrorResponse](t, ctx)

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
