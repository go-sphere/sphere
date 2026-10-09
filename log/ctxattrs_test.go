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
