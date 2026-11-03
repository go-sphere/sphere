package log

import (
	"fmt"
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

// namedNode is cyclic but renders itself through String, so fmt.Sprint never
// descends into next.
type namedNode struct {
	name string
	next *namedNode
}

func (n *namedNode) String() string { return "node:" + n.name }

type plainNode struct {
	name string
	next *plainNode
}

type errNode struct {
	next *errNode
}

func (*errNode) Error() string { return "errnode" }

type formatNode struct {
	next *formatNode
}

func (*formatNode) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte("formatnode")) }

// goStringNode only customizes %#v, which fmt.Sprint does not use, so its
// cycle is still reachable through formatting.
type goStringNode struct {
	next *goStringNode
}

func (*goStringNode) GoString() string { return "gostring" }

// TestFormatAnyTrustsFormattingMethods pins that a value fmt renders through
// Error, String or Format is not walked: fmt never descends into it, so a cycle
// behind such a method is harmless. A cyclic value without those methods is
// still rejected.
func TestFormatAnyTrustsFormattingMethods(t *testing.T) {
	c := &namedNode{name: "c"}
	c.next = c
	e := &errNode{}
	e.next = e
	f := &formatNode{}
	f.next = f
	p := &plainNode{name: "p"}
	p.next = p
	g := &goStringNode{}
	g.next = g

	for _, tc := range []struct {
		name string
		in   any
		want string // empty means the unformattable placeholder
	}{
		{name: "stringer", in: c, want: "node:c"},
		{name: "error", in: e, want: "errnode"},
		{name: "formatter", in: f, want: "formatnode"},
		{name: "typed nil stringer", in: (*namedNode)(nil), want: "<nil>"},
		{name: "slice element", in: []*namedNode{c}, want: "[node:c]"},
		{name: "map value", in: map[string]*namedNode{"k": c}, want: "map[k:node:c]"},
		{name: "exported struct field", in: struct{ N *namedNode }{c}, want: `"{node:c}"`},
		{name: "plain cycle", in: p},
		{name: "plain cycle in slice", in: []*plainNode{p}},
		{name: "unexported field is not formatted by method", in: struct{ n *plainNode }{p}},
		{name: "gostringer is not used by Sprint", in: g},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := formatAny(tc.in)
			if tc.want == "" {
				if !strings.Contains(got, "unformattable") {
					t.Fatalf("formatAny = %q, want the unformattable placeholder", got)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("formatAny = %q, want %q", got, tc.want)
			}
		})
	}
}
