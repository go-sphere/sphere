package storageerr

import (
	"errors"
	"net/http"
	"testing"

	"github.com/go-sphere/httpx"
)

// TestSentinelsCarryHTTPStatus pins that the shared storage sentinels remain
// comparable with errors.Is and carry the HTTP status their name implies.
// Drivers return these wrapped, and consumers map them back with errors.Is,
// so a driver that starts returning a plain error instead would silently
// change a 404 into a 500 at the HTTP layer.
func TestSentinelsCarryHTTPStatus(t *testing.T) {
	tests := []struct {
		name       string
		sentinel   error
		wantStatus int32
	}{
		{"not found", ErrNotFound, http.StatusNotFound},
		{"dest exists", ErrDestExists, http.StatusBadRequest},
		{"file name invalid", ErrFileNameInvalid, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := errors.Join(errors.New("driver detail"), tt.sentinel)
			if !errors.Is(wrapped, tt.sentinel) {
				t.Fatalf("errors.Is through a wrapped error = false, want true")
			}
			se, ok := errors.AsType[httpx.StatusError](tt.sentinel)
			if !ok {
				t.Fatal("sentinel does not implement httpx.StatusError")
			}
			if got := se.GetStatus(); got != tt.wantStatus {
				t.Fatalf("GetStatus() = %d, want %d", got, tt.wantStatus)
			}
		})
	}
}
