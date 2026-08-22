package httpz

import (
	"errors"
	"net/http"
	"testing"

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
