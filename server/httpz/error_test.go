package httpz

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

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

// TestAbortWithJsonError_UnclassifiedDoesNotLeak covers BOTH outbound fields.
// Asserting only on Error left the real leak uncovered: httpx.ParseError falls
// back to err.Error() for anything without a MessageError, so the raw text used
// to reach the client through Message while Error was correctly blank.
func TestAbortWithJsonError_UnclassifiedDoesNotLeak(t *testing.T) {
	prev := DebugMode()
	SetDebugMode(false)
	defer SetDebugMode(prev)

	const raw = "pq: password authentication failed for user \"admin\" (host 10.0.3.14:5432)"

	ctx := &fakeContext{}
	AbortWithJsonError(ctx, errors.New(raw))

	resp := ctx.body.(ErrorResponse)
	if resp.Error != "" {
		t.Fatalf("raw error leaked through Error: %q", resp.Error)
	}
	if resp.Message == raw {
		t.Fatalf("raw error leaked through Message: %q", resp.Message)
	}
	if resp.Message != http.StatusText(http.StatusInternalServerError) {
		t.Fatalf("expected generic status text, got %q", resp.Message)
	}
}

// TestAbortWithJsonError_WrappedUnclassifiedDoesNotLeak pins the same rule for a
// wrapped error, which is the shape service code actually produces.
func TestAbortWithJsonError_WrappedUnclassifiedDoesNotLeak(t *testing.T) {
	prev := DebugMode()
	SetDebugMode(false)
	defer SetDebugMode(prev)

	ctx := &fakeContext{}
	inner := errors.New("ent: constraint failed: UNIQUE constraint failed: users.email")
	AbortWithJsonError(ctx, fmt.Errorf("create user: %w", inner))

	resp := ctx.body.(ErrorResponse)
	if strings.Contains(resp.Message, "constraint") || strings.Contains(resp.Error, "constraint") {
		t.Fatalf("wrapped error leaked: message=%q error=%q", resp.Message, resp.Error)
	}
}

// TestAbortWithJsonError_EmptyMessageFallsBackToStatusText pins that a
// classified error carrying no user-facing text still produces a usable
// message rather than an empty string.
func TestAbortWithJsonError_EmptyMessageFallsBackToStatusText(t *testing.T) {
	prev := DebugMode()
	SetDebugMode(false)
	defer SetDebugMode(prev)

	ctx := &fakeContext{}
	AbortWithJsonError(ctx, httpx.UnauthorizedError(errors.New("token is expired")))

	resp := ctx.body.(ErrorResponse)
	if ctx.status != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, ctx.status)
	}
	if resp.Message != http.StatusText(http.StatusUnauthorized) {
		t.Fatalf("expected generic status text, got %q", resp.Message)
	}
	if resp.Error != "" {
		t.Fatalf("raw error leaked: %q", resp.Error)
	}
}

func TestAbortWithJsonError_DebugModeExposesError(t *testing.T) {
	prev := DebugMode()
	SetDebugMode(true)
	defer SetDebugMode(prev)

	ctx := &fakeContext{}
	AbortWithJsonError(ctx, errors.New("sensitive internal detail"))

	resp := ctx.body.(ErrorResponse)
	if resp.Error != "sensitive internal detail" {
		t.Fatalf("expected raw error in debug mode, got %q", resp.Error)
	}
}

func TestAbortWithJsonError_ClassifiedMessageReturned(t *testing.T) {
	prev := DebugMode()
	SetDebugMode(false)
	defer SetDebugMode(prev)

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

// TestAbortWithJsonErrorConcurrentConfig exercises the globals that
// AbortWithJsonError reads on every request against concurrent reconfiguration.
// It asserts nothing by itself; the race detector is the oracle.
func TestAbortWithJsonErrorConcurrentConfig(t *testing.T) {
	prevDebug := DebugMode()
	t.Cleanup(func() {
		SetDebugMode(prevDebug)
		SetDefaultErrorParser(httpx.ParseError)
	})

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Go(func() {
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			SetDebugMode(i%2 == 0)
			SetDefaultErrorParser(httpx.ParseError)
		}
	})
	for range 4 {
		wg.Go(func() {
			for range 500 {
				AbortWithJsonError(&fakeContext{}, errors.New("boom"))
			}
		})
	}

	time.Sleep(20 * time.Millisecond)
	close(stop)
	wg.Wait()
}

// TestSetDefaultErrorParserIgnoresNil pins that a nil parser cannot be installed,
// which would otherwise panic on the next request.
func TestSetDefaultErrorParserIgnoresNil(t *testing.T) {
	t.Cleanup(func() { SetDefaultErrorParser(httpx.ParseError) })

	SetDefaultErrorParser(nil)
	ctx := &fakeContext{}
	AbortWithJsonError(ctx, httpx.BadRequestError(errors.New("raw"), "friendly"))
	if ctx.status != http.StatusBadRequest {
		t.Fatalf("expected the previous parser to remain active, got status %d", ctx.status)
	}
}

func TestAbortWithJsonError_CustomParserMessageKept(t *testing.T) {
	prev := DebugMode()
	SetDebugMode(false)
	t.Cleanup(func() {
		SetDebugMode(prev)
		SetDefaultErrorParser(httpx.ParseError)
	})

	const userMsg = "name is required"
	raw := errors.New("validation error: name: required")
	SetDefaultErrorParser(func(err error) (int32, int32, string) {
		return 0, http.StatusBadRequest, userMsg
	})

	ctx := &fakeContext{}
	AbortWithJsonError(ctx, raw)
	if ctx.status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", ctx.status)
	}
	resp := ctx.body.(ErrorResponse)
	if resp.Message != userMsg {
		t.Fatalf("message = %q, want %q", resp.Message, userMsg)
	}
	if resp.Error != "" {
		t.Fatalf("leaked Error: %q", resp.Error)
	}
	if resp.Code != 0 {
		t.Fatalf("code = %d, want 0", resp.Code)
	}
}

func TestAbortWithJsonError_ParserEchoingRawErrorIsSanitized(t *testing.T) {
	prev := DebugMode()
	SetDebugMode(false)
	t.Cleanup(func() {
		SetDebugMode(prev)
		SetDefaultErrorParser(httpx.ParseError)
	})

	raw := errors.New("pq: password authentication failed")
	SetDefaultErrorParser(func(err error) (int32, int32, string) {
		return 0, http.StatusInternalServerError, err.Error()
	})

	ctx := &fakeContext{}
	AbortWithJsonError(ctx, raw)
	resp := ctx.body.(ErrorResponse)
	if resp.Message != http.StatusText(http.StatusInternalServerError) {
		t.Fatalf("message = %q", resp.Message)
	}
}

func TestAbortWithJsonError_OutOfRangeStatusClamped(t *testing.T) {
	t.Cleanup(func() {
		SetDefaultErrorParser(httpx.ParseError)
	})

	for _, invalidStatus := range []int32{0, -1, 50, 99, 600, 700, 1000} {
		SetDefaultErrorParser(func(err error) (int32, int32, string) {
			return 0, invalidStatus, "invalid status message"
		})
		ctx := &fakeContext{}
		AbortWithJsonError(ctx, errors.New("sample error"))
		if ctx.status != http.StatusInternalServerError {
			t.Errorf("status %d was not clamped to %d, got %d", invalidStatus, http.StatusInternalServerError, ctx.status)
		}
	}
}

// TestStressAbortWithJsonError_ExtremeStatuses tests extreme status codes and clamping
func TestStressAbortWithJsonError_ExtremeStatuses(t *testing.T) {
	t.Cleanup(func() {
		SetDefaultErrorParser(httpx.ParseError)
	})

	extremeStatuses := []int32{
		-1000000, -500, -1, 0, 1, 50, 99,
		600, 601, 700, 999, 1000, 50000,
	}

	for _, st := range extremeStatuses {
		SetDefaultErrorParser(func(err error) (int32, int32, string) {
			return 0, st, "test message"
		})

		ctx := &stressFakeContext{}
		AbortWithJsonError(ctx, errors.New("some error"))

		if ctx.status != http.StatusInternalServerError {
			t.Errorf("extreme status %d was not clamped to 500, got %d", st, ctx.status)
		}
	}
}
