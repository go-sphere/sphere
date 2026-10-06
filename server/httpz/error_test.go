package httpz

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/httpxmock"
	"github.com/go-sphere/sphere/storage/storageerr"
)

// errorBody returns the envelope AbortWithJsonError handed to JSON. It asserts
// on the value rather than on the encoded body, so two structs sharing a JSON
// shape stay distinguishable.
func errorBody(t *testing.T, ctx *httpxmock.Context) ErrorResponse {
	t.Helper()
	v, ok := ctx.LastJSON()
	if !ok {
		t.Fatalf("no JSON response was written")
	}
	resp, ok := v.(ErrorResponse)
	if !ok {
		t.Fatalf("body type = %T, want ErrorResponse", v)
	}
	return resp
}

func TestAbortWithJsonError_NilDoesNotPanic(t *testing.T) {
	ctx := httpxmock.New(nil)
	// Must not panic on a nil error interface.
	AbortWithJsonError(ctx, nil)
	if ctx.StatusCode() != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, ctx.StatusCode())
	}
	resp := errorBody(t, ctx)
	if resp.Error != "" {
		t.Fatalf("expected empty Error field for nil error, got %q", resp.Error)
	}
}

// TestAbortWithJsonError_UnclassifiedDoesNotLeak covers BOTH outbound fields.
// Asserting only on Error left the real leak uncovered: httpx.ParseError used
// to fall back to err.Error() for anything without a MessageError, so the raw
// text reached the client through Message while Error was correctly blank. It
// returns an empty message instead from v0.0.5, and this test is what keeps
// that regression from coming back through a custom parser installed as the
// default.
func TestAbortWithJsonError_UnclassifiedDoesNotLeak(t *testing.T) {
	prev := DebugMode()
	SetDebugMode(false)
	defer SetDebugMode(prev)

	const raw = "pq: password authentication failed for user \"admin\" (host 10.0.3.14:5432)"

	ctx := httpxmock.New(nil)
	AbortWithJsonError(ctx, errors.New(raw))

	resp := errorBody(t, ctx)
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

	ctx := httpxmock.New(nil)
	inner := errors.New("ent: constraint failed: UNIQUE constraint failed: users.email")
	AbortWithJsonError(ctx, fmt.Errorf("create user: %w", inner))

	resp := errorBody(t, ctx)
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

	ctx := httpxmock.New(nil)
	AbortWithJsonError(ctx, httpx.UnauthorizedError(errors.New("token is expired")))

	resp := errorBody(t, ctx)
	if ctx.StatusCode() != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, ctx.StatusCode())
	}
	if resp.Message != http.StatusText(http.StatusUnauthorized) {
		t.Fatalf("expected generic status text, got %q", resp.Message)
	}
	if resp.Error != "" {
		t.Fatalf("raw error leaked: %q", resp.Error)
	}
}

func TestAbortWithJsonError_CustomParserPreservesMessageAndCode(t *testing.T) {
	t.Cleanup(func() { SetDefaultErrorParser(ParseError) })
	SetDefaultErrorParser(func(error) (int32, int32, string) {
		return 1001, http.StatusNotFound, "user not found"
	})

	ctx := httpxmock.New(nil)
	AbortWithJsonError(ctx, errors.New("not found"))
	if ctx.StatusCode() != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", ctx.StatusCode())
	}
	resp := errorBody(t, ctx)
	if resp.Message != "user not found" || resp.Code != 1001 {
		t.Fatalf("response = %+v, want custom message and code", resp)
	}
}

func TestAbortWithJsonError_DebugModeExposesError(t *testing.T) {
	prev := DebugMode()
	SetDebugMode(true)
	defer SetDebugMode(prev)

	ctx := httpxmock.New(nil)
	AbortWithJsonError(ctx, errors.New("sensitive internal detail"))

	resp := errorBody(t, ctx)
	if resp.Error != "sensitive internal detail" {
		t.Fatalf("expected raw error in debug mode, got %q", resp.Error)
	}
}

func TestAbortWithJsonError_ClassifiedMessageReturned(t *testing.T) {
	prev := DebugMode()
	SetDebugMode(false)
	defer SetDebugMode(prev)

	ctx := httpxmock.New(nil)
	err := httpx.BadRequestError(errors.New("raw sql detail"), "please provide a valid id")
	AbortWithJsonError(ctx, err)

	resp := errorBody(t, ctx)
	if ctx.StatusCode() != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, ctx.StatusCode())
	}
	if resp.Message != "please provide a valid id" {
		t.Fatalf("expected user-facing message returned, got %q", resp.Message)
	}
	if resp.Error != "" {
		t.Fatalf("expected empty Error field (raw detail must not leak), got %q", resp.Error)
	}
}

// TestAbortWithJsonErrorConcurrentConfig exercises requests alongside global
// reconfiguration and verifies every response remains structurally valid.
func TestAbortWithJsonErrorConcurrentConfig(t *testing.T) {
	prevDebug := DebugMode()
	t.Cleanup(func() {
		SetDebugMode(prevDebug)
		SetDefaultErrorParser(ParseError)
	})

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Go(func() {
		<-start
		for i := range 1_000 {
			SetDebugMode(i%2 == 0)
			SetDefaultErrorParser(ParseError)
		}
	})
	for range 4 {
		wg.Go(func() {
			<-start
			for range 250 {
				ctx := httpxmock.New(nil)
				AbortWithJsonError(ctx, errors.New("boom"))
				if ctx.StatusCode() != http.StatusInternalServerError {
					t.Errorf("status = %d, want %d", ctx.StatusCode(), http.StatusInternalServerError)
				}
				if v, ok := ctx.LastJSON(); !ok {
					t.Error("no JSON response was written")
				} else if _, ok := v.(ErrorResponse); !ok {
					t.Errorf("body type = %T, want ErrorResponse", v)
				}
			}
		})
	}

	close(start)
	wg.Wait()
}

// TestSetDefaultErrorParserIgnoresNil pins that a nil parser cannot be installed,
// which would otherwise panic on the next request.
func TestSetDefaultErrorParserIgnoresNil(t *testing.T) {
	t.Cleanup(func() { SetDefaultErrorParser(ParseError) })

	SetDefaultErrorParser(nil)
	ctx := httpxmock.New(nil)
	AbortWithJsonError(ctx, httpx.BadRequestError(errors.New("raw"), "friendly"))
	if ctx.StatusCode() != http.StatusBadRequest {
		t.Fatalf("expected the previous parser to remain active, got status %d", ctx.StatusCode())
	}
}

func TestAbortWithJsonError_CustomParserMessageKept(t *testing.T) {
	prev := DebugMode()
	SetDebugMode(false)
	t.Cleanup(func() {
		SetDebugMode(prev)
		SetDefaultErrorParser(ParseError)
	})

	const userMsg = "name is required"
	raw := errors.New("validation error: name: required")
	SetDefaultErrorParser(func(err error) (int32, int32, string) {
		return 0, http.StatusBadRequest, userMsg
	})

	ctx := httpxmock.New(nil)
	AbortWithJsonError(ctx, raw)
	if ctx.StatusCode() != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", ctx.StatusCode())
	}
	resp := errorBody(t, ctx)
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

// TestAbortWithJsonError_ParserEchoingRawErrorIsKept pins the v0.0.5 contract
// change: this package no longer second-guesses a custom parser by dropping a
// message that equals err.Error(). That guard existed because httpx.ParseError
// used to fall back to the raw text for an unclassified error; it returns an
// empty message now, so the guard only fired on a parser that deliberately
// returned the raw string — dropping the message it had just chosen. A parser
// is trusted setup code, and leak protection belongs to the default parser
// (TestAbortWithJsonError_UnclassifiedDoesNotLeak).
func TestAbortWithJsonError_ParserEchoingRawErrorIsKept(t *testing.T) {
	prev := DebugMode()
	SetDebugMode(false)
	t.Cleanup(func() {
		SetDebugMode(prev)
		SetDefaultErrorParser(ParseError)
	})

	raw := errors.New("pq: password authentication failed")
	SetDefaultErrorParser(func(err error) (int32, int32, string) {
		return 0, http.StatusInternalServerError, err.Error()
	})

	ctx := httpxmock.New(nil)
	AbortWithJsonError(ctx, raw)
	resp := errorBody(t, ctx)
	if resp.Message != raw.Error() {
		t.Fatalf("message = %q, want the parser's own %q", resp.Message, raw.Error())
	}
}

func TestAbortWithJsonError_OutOfRangeStatusClamped(t *testing.T) {
	t.Cleanup(func() {
		SetDefaultErrorParser(ParseError)
	})

	for _, invalidStatus := range []int32{0, -1, 50, 99, 600, 700, 1000} {
		SetDefaultErrorParser(func(err error) (int32, int32, string) {
			return 0, invalidStatus, "invalid status message"
		})
		ctx := httpxmock.New(nil)
		AbortWithJsonError(ctx, errors.New("sample error"))
		if ctx.StatusCode() != http.StatusInternalServerError {
			t.Errorf("status %d was not clamped to %d, got %d", invalidStatus, http.StatusInternalServerError, ctx.StatusCode())
		}
	}
}

// TestStorageSentinelsRenderWithHTTPStatus is the contract that replaced the
// HTTP status the storage sentinels used to carry themselves (they were
// httpx.NotFoundError/BadRequestError values until v0.0.7). A storage error
// returned straight from a handler must still render as 404/400 through the
// default parser, through ParseError called directly, and through a custom
// parser that delegates to ParseError — the shape the layout templates use.
// A regression here turns a missing file into a 500.
func TestStorageSentinelsRenderWithHTTPStatus(t *testing.T) {
	prevDebug := DebugMode()
	SetDebugMode(false)
	t.Cleanup(func() {
		SetDebugMode(prevDebug)
		SetDefaultErrorParser(ParseError)
	})

	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"not found", storageerr.ErrNotFound, http.StatusNotFound},
		{"wrapped not found", fmt.Errorf("download avatar: %w", storageerr.ErrNotFound), http.StatusNotFound},
		{"dest exists", storageerr.ErrDestExists, http.StatusBadRequest},
		{"file name invalid", storageerr.ErrFileNameInvalid, http.StatusBadRequest},
		{"joined file name invalid", errors.Join(errors.New("driver detail"), storageerr.ErrFileNameInvalid), http.StatusBadRequest},
		{"explicit status wins", httpx.InternalServerError(storageerr.ErrNotFound), http.StatusInternalServerError},
	}

	parsers := []struct {
		name   string
		parser ErrorParser
	}{
		{"default parser", ParseError},
		{"custom parser delegating to ParseError", func(err error) (int32, int32, string) {
			if errors.Is(err, errCustomOnly) {
				return 0, http.StatusTeapot, ""
			}
			return ParseError(err)
		}},
	}

	for _, p := range parsers {
		for _, tt := range tests {
			t.Run(p.name+"/"+tt.name, func(t *testing.T) {
				SetDefaultErrorParser(p.parser)

				code, status, _ := ParseError(tt.err)
				if int(status) != tt.wantStatus || code != 0 {
					t.Fatalf("ParseError() = (code %d, status %d), want (0, %d)", code, status, tt.wantStatus)
				}

				ctx := httpxmock.New(nil)
				AbortWithJsonError(ctx, tt.err)
				if ctx.StatusCode() != tt.wantStatus {
					t.Fatalf("rendered status = %d, want %d", ctx.StatusCode(), tt.wantStatus)
				}
				resp := errorBody(t, ctx)
				if resp.Code != 0 || resp.Message != http.StatusText(tt.wantStatus) {
					t.Fatalf("response = %+v, want code 0 and message %q", resp, http.StatusText(tt.wantStatus))
				}
			})
		}
	}
}

var errCustomOnly = errors.New("handled by the custom parser only")
