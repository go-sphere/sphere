package idgenerator

import (
	"math"
	"testing"
)

func TestNextId(t *testing.T) {
	t.Log(NextId())
	t.Log(math.MaxInt32)
	t.Log(math.MaxInt64)
}

// TestParseWorkerID pins two rules that a fixed-value fallback silently broke.
//
// Zero must be accepted: the underlying generator's range is [0, 63], and
// deriving WORKER_ID from a StatefulSet pod ordinal makes "0" the single most
// common value in practice. Treating it as invalid mapped pod-0 onto the same
// worker ID as pod-1 and produced colliding IDs.
//
// A malformed value must fail rather than fall back, for the same reason: any
// fixed substitute is shared with whichever replica legitimately holds it.
func TestParseWorkerID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    uint16
		wantErr bool
	}{
		{name: "unset uses the single-instance default", raw: "", want: defaultWorkerID},
		{name: "zero is a valid worker id", raw: "0", want: 0},
		{name: "ordinary value is used", raw: "42", want: 42},
		{name: "max worker id is valid", raw: "63", want: 63},
		{name: "above max is rejected", raw: "64", wantErr: true},
		{name: "above uint16 is rejected", raw: "65536", wantErr: true},
		{name: "non numeric is rejected", raw: "abc", wantErr: true},
		{name: "negative is rejected", raw: "-1", wantErr: true},
		{name: "trailing space is rejected", raw: "1 ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseWorkerID(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseWorkerID(%q) = (%d, nil), want an error", tt.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseWorkerID(%q) returned unexpected error: %v", tt.raw, err)
			}
			if got != tt.want {
				t.Errorf("parseWorkerID(%q) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}

// TestDistinctWorkerIDsProduceDistinctIDs is the property the worker ID exists
// to guarantee: two replicas configured with different worker IDs must never
// emit the same ID. Worker IDs 0 and 1 are the pod-0/pod-1 pair that used to
// collapse onto a single generator.
func TestDistinctWorkerIDsProduceDistinctIDs(t *testing.T) {
	const perWorker = 2000

	seen := make(map[int64]uint16, perWorker*2)
	for _, workerID := range []uint16{0, 1} {
		next := NewIdGenerator(workerID)
		for range perWorker {
			id := next()
			if other, dup := seen[id]; dup {
				t.Fatalf("worker %d and worker %d both produced id %d", other, workerID, id)
			}
			seen[id] = workerID
		}
	}
}
