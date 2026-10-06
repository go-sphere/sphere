package storageerr

import (
	"errors"
	"testing"
)

// TestSentinelsMatchThroughWrapping pins that the shared storage sentinels
// stay comparable with errors.Is through the wrapping drivers apply. The HTTP
// status they render with is pinned next to the mapping, in server/httpz.
func TestSentinelsMatchThroughWrapping(t *testing.T) {
	for _, sentinel := range []error{ErrNotFound, ErrDestExists, ErrFileNameInvalid} {
		t.Run(sentinel.Error(), func(t *testing.T) {
			wrapped := errors.Join(errors.New("driver detail"), sentinel)
			if !errors.Is(wrapped, sentinel) {
				t.Fatalf("errors.Is through a wrapped error = false, want true")
			}
		})
	}
}
