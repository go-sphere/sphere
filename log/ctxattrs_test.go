package log

import (
	"context"
	"testing"
)

type ctxAttrBackend struct {
	Backend
	attrs []Attr
}

func (b *ctxAttrBackend) Log(_ context.Context, _ Level, _ string, attrs ...Attr) {
	b.attrs = attrs
}

func TestContextWithAttrsAccumulates(t *testing.T) {
	t.Parallel()
	ctx := ContextWithAttrs(t.Context(), String("a", "1"))
	ctx2 := ContextWithAttrs(ctx, String("b", "2"))
	if got := AttrsFromContext(ctx); len(got) != 1 {
		t.Fatalf("parent mutated: %v", got)
	}
	got := AttrsFromContext(ctx2)
	if len(got) != 2 || got[0].Key != "a" || got[1].Key != "b" {
		t.Fatalf("attrs = %v", got)
	}
	if AttrsFromContext(t.Context()) != nil {
		t.Fatal("expected nil without attrs")
	}
}

func TestCoreLoggerAppendsContextAttrsOnlyOnContextMethods(t *testing.T) {
	t.Parallel()
	b := &ctxAttrBackend{}
	l := NewLogger(b)
	ctx := ContextWithAttrs(t.Context(), String("request_id", "r1"))

	l.InfoContext(ctx, "m", String("k", "v"))
	if len(b.attrs) != 2 || b.attrs[0].Key != "k" || b.attrs[1].Key != "request_id" {
		t.Fatalf("attrs = %v", b.attrs)
	}
	l.Info("m", String("k", "v"))
	if len(b.attrs) != 1 {
		t.Fatalf("plain method attrs = %v", b.attrs)
	}
}

// TestContextAttrsYieldToCallSiteAttrs pins the precedence between the two
// attr sources: a key set at the call site wins over the same key attached to
// the context, the entry carries it once, and a later ContextWithAttrs wins
// over an earlier one. The context-merge wrapper must keep that outcome.
func TestContextAttrsYieldToCallSiteAttrs(t *testing.T) {
	t.Parallel()
	ctx := ContextWithAttrs(t.Context(), String("request_id", "ctx"), String("user", "old"))
	ctx = ContextWithAttrs(ctx, String("user", "new"))

	b := &ctxAttrBackend{}
	merged := WrapBackendWithContextMerge(b, func(context.Context) []Attr {
		return []Attr{String("trace_id", "t")}
	})
	for name, backend := range map[string]Backend{"plain": b, "merge": merged} {
		NewLogger(backend).InfoContext(ctx, "m", String("request_id", "explicit"))
		got := map[string]string{}
		for _, a := range b.attrs {
			if _, dup := got[a.Key]; dup {
				t.Fatalf("%s: key %q emitted twice: %v", name, a.Key, b.attrs)
			}
			got[a.Key] = a.Value.String()
		}
		if got["request_id"] != "explicit" || got["user"] != "new" {
			t.Fatalf("%s: attrs = %v, want request_id=explicit user=new", name, b.attrs)
		}
	}
}

func TestWithContextAttrsSmallAndLarge(t *testing.T) {
	t.Parallel()

	t.Run("linear scan path (<= 8 attrs)", func(t *testing.T) {
		ctx := ContextWithAttrs(t.Context(),
			String("a", "1"),
			String("b", "2"),
			String("a", "3"),
			String("", "inline"),
		)
		got := withContextAttrs(ctx, []Attr{String("b", "callsite")})
		if len(got) != 3 {
			t.Fatalf("len = %d, got %v", len(got), got)
		}
		if got[0].Key != "b" || got[0].Value.String() != "callsite" {
			t.Fatalf("callsite attr = %v", got[0])
		}
		if got[1].Key != "a" || got[1].Value.String() != "3" {
			t.Fatalf("overridden ctx attr = %v", got[1])
		}
		if got[2].Key != "" || got[2].Value.String() != "inline" {
			t.Fatalf("inline attr = %v", got[2])
		}
	})

	t.Run("map path (> 8 attrs)", func(t *testing.T) {
		var extra []Attr
		for i := range 10 {
			extra = append(extra, Int("k"+string(rune('0'+i)), i))
		}
		extra = append(extra, Int("k0", 99))
		ctx := ContextWithAttrs(t.Context(), extra...)
		got := withContextAttrs(ctx, []Attr{Int("k1", 100)})
		if len(got) != 10 {
			t.Fatalf("len = %d, want 10: %v", len(got), got)
		}
		vals := map[string]int64{}
		for _, a := range got {
			vals[a.Key] = a.Value.Int64()
		}
		if vals["k0"] != 99 || vals["k1"] != 100 {
			t.Fatalf("vals = %v", vals)
		}
	})
}
