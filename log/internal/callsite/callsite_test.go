package callsite

import "testing"

func TestInTree(t *testing.T) {
	for _, tc := range []struct {
		function string
		want     bool
	}{
		{root + ".Info", true},
		{root + ".(*coreLogger).Info", true},
		{root + ".(*MultiBackend).Log", true},
		{root + ".Info.func1", true},
		{root + "/zapx.(*Backend).Log", true},
		{root + "/logbuffer.(*Buffer).Log", true},
		{root + "/internal/callsite.Frame", true},
		{root + "_test.TestCaller", false},
		{root + "/zapx_test.TestCaller", false},
		{root + "x.Info", false},
		{"main.main", false},
		{"runtime.goexit", false},
		{"", false},
	} {
		if got := inTree(tc.function); got != tc.want {
			t.Errorf("inTree(%q) = %v, want %v", tc.function, got, tc.want)
		}
	}
}
