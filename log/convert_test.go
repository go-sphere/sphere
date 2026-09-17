package log

import (
	"log/slog"
	"strings"
	"testing"
)

// selfReferentialValuer resolves to a group that contains itself. The cycle
// runs through slog.KindGroup, never through a reflect container, so only the
// group path's depth bound can cut it.
type selfReferentialValuer struct{}

func (selfReferentialValuer) LogValue() slog.Value {
	return slog.GroupValue(slog.Any("self", selfReferentialValuer{}))
}

// TestFormatSlogValueBoundsGroupCycles pins that a LogValuer resolving to a
// group containing itself is reported as unformattable instead of recursing
// until the goroutine stack is exhausted, which is fatal to the process and
// cannot be recovered.
func TestFormatSlogValueBoundsGroupCycles(t *testing.T) {
	got := formatSlogValue(Any("g", selfReferentialValuer{}).Value)
	if !strings.Contains(got, "unformattable") {
		t.Fatalf("formatSlogValue = %q, want the placeholder for a cyclic group", got)
	}
}

// TestFormatSlogValueBoundsDeepGroups pins the other half of the bound: a group
// nested deeper than maxFormatDepth is truncated rather than formatted.
func TestFormatSlogValueBoundsDeepGroups(t *testing.T) {
	attr := String("leaf", "value")
	for range maxFormatDepth + 1 {
		attr = Attr{Key: "g", Value: slog.GroupValue(attr)}
	}
	if got := formatSlogValue(attr.Value); !strings.Contains(got, "unformattable") {
		t.Fatalf("formatSlogValue = %q, want the placeholder for a too-deep group", got)
	}
}

// TestFormatAnyAliasingSubSliceIsNotACycle pins that a slice's identity is not
// just the backing-array base: a sub-slice that merely aliases an ancestor
// shares that base without forming a cycle, and fmt.Sprint terminates on it.
func TestFormatAnyAliasingSubSliceIsNotACycle(t *testing.T) {
	s := make([]any, 3)
	s[2] = s[0:1]

	if got := formatAny(s); strings.Contains(got, "unformattable") {
		t.Fatalf("formatAny = %q, want the aliasing slice formatted", got)
	}

	cyclic := make([]any, 1)
	cyclic[0] = cyclic
	if got := formatAny(cyclic); !strings.Contains(got, "unformattable") {
		t.Fatalf("formatAny = %q, want a genuine self-reference rejected", got)
	}
}

// TestQuoteIfNeededEscapesStructuralAndControlCharacters pins that values which
// would otherwise change the shape of a logfmt line — or inject terminal
// escapes — are quoted. A group value must not be indistinguishable from two
// members, and a control character must not be able to forge a line.
func TestQuoteIfNeededEscapesStructuralAndControlCharacters(t *testing.T) {
	for _, in := range []string{"a,b", "a}b", "a{b", "a\nb", "a=b", "a b", "\x1b[31mred", "\x00", `a"b`, ""} {
		if got := quoteIfNeeded(in); got == in {
			t.Errorf("quoteIfNeeded(%q) = %q, want it escaped", in, got)
		}
	}
	if got := quoteIfNeeded("plain"); got != "plain" {
		t.Errorf("quoteIfNeeded(plain) = %q, want it unchanged", got)
	}
}

// TestFormatGroupQuotesKeysAndValues pins that group keys take the same escaping
// as values, so a key containing a delimiter cannot break the group structure.
func TestFormatGroupQuotesKeysAndValues(t *testing.T) {
	got := formatSlogValue(Group("g", String("k,x", "a,b")).Value)
	if !strings.Contains(got, `"k,x"`) || !strings.Contains(got, `"a,b"`) {
		t.Fatalf("formatSlogValue = %q, want key and value quoted", got)
	}
}
