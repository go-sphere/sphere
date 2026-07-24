package auth

import (
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/server/auth/authorizer"
)

func TestUnauthorizedErrorPreservesUserMessage(t *testing.T) {
	t.Parallel()

	err := unauthorizedError(authorizer.TokenNotFoundError)
	code, status, message := httpx.ParseError(err)
	wantCode, wantStatus, wantMessage := httpx.ParseError(authorizer.TokenNotFoundError)

	if code != wantCode || status != wantStatus || message != wantMessage {
		t.Fatalf(
			"unauthorized error mismatch: got=(%d, %d, %q) want=(%d, %d, %q)",
			code,
			status,
			message,
			wantCode,
			wantStatus,
			wantMessage,
		)
	}
}
