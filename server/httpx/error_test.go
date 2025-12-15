package httpx

import (
	"errors"
	"testing"

	"github.com/go-sphere/sphere/core/errors/statuserr"
)

func TestParseError(t *testing.T) {
	err := statuserr.NewError(400, 1234, "test", errors.New("detail"))
	code, status, message := ParseError(err)
	if code != 1234 {
		t.Errorf("expected code 1234, got %d", code)
	}
	if status != 400 {
		t.Errorf("expected status 400, got %d", status)
	}
	if message != "test" {
		t.Errorf("expected message 'test', got '%s'", message)
	}
	if err.Error() != "detail" {
		t.Errorf("expected error 'detail', got '%s'", err.Error())
	}
}
