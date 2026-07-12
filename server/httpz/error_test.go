package httpz

import (
	"errors"
	"net/http"
	"testing"

	"github.com/go-sphere/httpx"
)

// httpxContext aliases httpx.Context so the embedded field name does not collide
// with the interface's own Context() method (which would shadow it).
type httpxContext = httpx.Context

// fakeContext embeds httpx.Context so it satisfies the full interface while only
// overriding JSON, which is the sole method AbortWithJsonError depends on.
type fakeContext struct {
	httpxContext
	status int
	body   any
}

func (f *fakeContext) JSON(code int, v any) error {
	f.status = code
	f.body = v
	return nil
}

func TestAbortWithJsonError_NilDoesNotPanic(t *testing.T) {
	ctx := &fakeContext{}
	// Must not panic on a nil error interface.
	AbortWithJsonError(ctx, nil)
	if ctx.status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, ctx.status)
	}
	resp, ok := ctx.body.(ErrorResponse)
	if !ok {
		t.Fatalf("expected ErrorResponse body, got %T", ctx.body)
	}
	if resp.Error != "" {
		t.Fatalf("expected empty Error field for nil error, got %q", resp.Error)
	}
}

func TestAbortWithJsonError_UnclassifiedDoesNotLeak(t *testing.T) {
	prev := DebugMode
	DebugMode = false
	defer func() { DebugMode = prev }()

	ctx := &fakeContext{}
	AbortWithJsonError(ctx, errors.New("sensitive internal detail"))

	resp := ctx.body.(ErrorResponse)
	if resp.Error != "" {
		t.Fatalf("raw error leaked to client: %q", resp.Error)
	}
}

func TestAbortWithJsonError_DebugModeExposesError(t *testing.T) {
	prev := DebugMode
	DebugMode = true
	defer func() { DebugMode = prev }()

	ctx := &fakeContext{}
	AbortWithJsonError(ctx, errors.New("sensitive internal detail"))

	resp := ctx.body.(ErrorResponse)
	if resp.Error != "sensitive internal detail" {
		t.Fatalf("expected raw error in debug mode, got %q", resp.Error)
	}
}

func TestAbortWithJsonError_ClassifiedMessageReturned(t *testing.T) {
	prev := DebugMode
	DebugMode = false
	defer func() { DebugMode = prev }()

	ctx := &fakeContext{}
	err := httpx.BadRequestError(errors.New("raw sql detail"), "please provide a valid id")
	AbortWithJsonError(ctx, err)

	resp := ctx.body.(ErrorResponse)
	if ctx.status != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, ctx.status)
	}
	if resp.Message != "please provide a valid id" {
		t.Fatalf("expected user-facing message returned, got %q", resp.Message)
	}
	if resp.Error != "" {
		t.Fatalf("expected empty Error field (raw detail must not leak), got %q", resp.Error)
	}
}
