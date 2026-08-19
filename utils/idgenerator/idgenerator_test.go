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

func TestParseWorkerID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		raw    string
		want   uint16
		wantOK bool
	}{
		{name: "unset uses the default without warning", raw: "", want: defaultWorkerID, wantOK: true},
		{name: "valid value is used", raw: "42", want: 42, wantOK: true},
		{name: "max uint16 is valid", raw: "65535", want: 65535, wantOK: true},
		{name: "zero is rejected", raw: "0", want: defaultWorkerID},
		{name: "out of range is rejected", raw: "65536", want: defaultWorkerID},
		{name: "non numeric is rejected", raw: "abc", want: defaultWorkerID},
		{name: "negative is rejected", raw: "-1", want: defaultWorkerID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := parseWorkerID(tt.raw)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("parseWorkerID(%q) = (%d, %v), want (%d, %v)", tt.raw, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
