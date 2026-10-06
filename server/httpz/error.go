package httpz

import (
	"errors"
	"net/http"
	"sync/atomic"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/storage/storageerr"
)

// ErrorParser is a function type that extracts error information for HTTP responses.
// It returns, in order, the application error code, the HTTP status code, and
// the user-facing message for err. AbortWithJsonError replaces a status outside
// 100–599 with 500, an empty message with the status text, and a code equal to
// the status (when err carries no httpx.CodeError) with 0.
type ErrorParser func(error) (int32, int32, string)

// Both globals are read by AbortWithJsonError on every request and may be written
// from application setup code, so they are swapped atomically rather than left as
// plain variables.
var (
	defaultErrorParser atomic.Pointer[ErrorParser]
	debugMode          atomic.Bool
)

func init() {
	var parser ErrorParser = ParseError
	defaultErrorParser.Store(&parser)
}

// ParseError is the default ErrorParser: httpx.ParseError plus the HTTP
// status of sentinel errors defined below the transport layer, which carry no
// status of their own. It maps (via errors.Is):
//
//   - storageerr.ErrNotFound to 404;
//   - storageerr.ErrDestExists and storageerr.ErrFileNameInvalid to 400.
//
// An httpx.StatusError anywhere in err's chain takes precedence, so a handler
// that wraps a storage error with an explicit status keeps that status.
//
// A custom parser installed with SetDefaultErrorParser replaces this one
// entirely. It must fall back to ParseError, not httpx.ParseError, for errors
// it does not handle itself — otherwise a missing storage key renders as 500:
//
//	httpz.SetDefaultErrorParser(func(err error) (int32, int32, string) {
//		if ve, ok := errors.AsType[*protovalidate.ValidationError](err); ok {
//			return 0, http.StatusBadRequest, ve.Error()
//		}
//		return httpz.ParseError(err)
//	})
func ParseError(err error) (code int32, status int32, message string) {
	code, status, message = httpx.ParseError(err)
	if _, ok := errors.AsType[httpx.StatusError](err); ok {
		return code, status, message
	}
	switch {
	case errors.Is(err, storageerr.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, storageerr.ErrDestExists), errors.Is(err, storageerr.ErrFileNameInvalid):
		status = http.StatusBadRequest
	}
	return code, status, message
}

// SetDefaultErrorParser sets the global error parser function for the package.
// This parser will be used by AbortWithJsonError when no specific parser is provided.
// A nil parser is ignored, leaving the current one in place. The parser
// replaces ParseError entirely; fall back to ParseError for unhandled errors.
// The parser is process-wide and swapped atomically, so it is safe to call
// while requests are served; it affects every later AbortWithJsonError call
// and SSE error event.
func SetDefaultErrorParser(parser ErrorParser) {
	if parser == nil {
		return
	}
	defaultErrorParser.Store(&parser)
}

// SetDebugMode controls whether raw error details are exposed to clients.
// When disabled (the default), the ErrorResponse.Error field is left empty and
// ErrorResponse.Message is replaced by the generic status text unless the error
// carries an explicit user-facing message, so that unclassified error strings
// and panic details are not leaked to callers. Enable it only in development to
// include err.Error() in responses. The setting is process-wide and safe for
// concurrent use.
func SetDebugMode(enabled bool) {
	debugMode.Store(enabled)
}

// DebugMode reports whether raw error details are exposed to clients.
func DebugMode() bool {
	return debugMode.Load()
}

// AbortWithJsonError writes an ErrorResponse and is the error path for
// WithJson/WithText/WithRecover. Status is clamped to 100–599 (invalid values
// become 500). A nil err logs a warning and writes a 500.
//
// Its signature matches httpx.ErrorHandler, so it can be installed as an
// engine's error handler (for example stdx.WithErrorHandler) to render errors
// returned by middleware with the same envelope.
func AbortWithJsonError(ctx httpx.Context, err error) {
	if err == nil {
		log.Warn("AbortWithJsonError called with nil error")
		_ = ctx.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    0,
			Message: http.StatusText(http.StatusInternalServerError),
		})
		return
	}
	status, resp := buildErrorResponse(err)
	_ = ctx.JSON(status, resp)
}

// ErrorStatus reports the HTTP status AbortWithJsonError writes for err: the
// status from the parser installed with SetDefaultErrorParser (ParseError by
// default), with values outside 100–599 replaced by 500. A nil err reports 500.
// Middleware that logs or meters a handler error should use it so the
// recorded status matches the response.
func ErrorStatus(err error) int {
	if err == nil {
		return http.StatusInternalServerError
	}
	status, _ := buildErrorResponse(err)
	return status
}

// buildErrorResponse maps err to an HTTP status and the standard
// ErrorResponse envelope using the configured parser and debug mode. It is
// shared by AbortWithJsonError and the in-stream SSE error frame so both
// paths render errors identically.
func buildErrorResponse(err error) (int, ErrorResponse) {
	code, status, message := (*defaultErrorParser.Load())(err)
	if status < 100 || status > 599 {
		status = http.StatusInternalServerError
	}
	// Unify the semantics of ErrorResponse.Code: it carries an application
	// specific error code and is 0 when the error is unclassified, so the HTTP
	// status alone carries the transport-level semantics. A parser that reports
	// code == status has conflated the two and is normalized to 0; any other
	// code is taken as the parser's deliberate application code.
	if _, ok := errors.AsType[httpx.CodeError](err); !ok && code == status {
		code = 0
	}
	// Message is user-facing. Prefer an explicit MessageError, otherwise take the
	// parser's message and fall back to the generic status text only when it is
	// empty. A custom parser is trusted application code and its message is
	// authoritative: it is how protovalidate (or similar) is mapped without
	// wrapping. The raw err.Error() no longer needs to be filtered out here —
	// httpx.ParseError, which the default ParseError builds on, returns an
	// empty message for an error that carries no MessageError instead of its
	// text (httpx v0.0.5), so driver and database strings cannot reach Message
	// through it. Pinned by
	// TestAbortWithJsonError_UnclassifiedDoesNotLeak.
	if me, ok := errors.AsType[httpx.MessageError](err); ok && me.GetMessage() != "" {
		message = me.GetMessage()
	} else if message == "" {
		message = http.StatusText(int(status))
	}
	resp := ErrorResponse{
		Code:    int(code),
		Message: message,
	}
	if debugMode.Load() {
		resp.Error = err.Error()
	}
	return int(status), resp
}
