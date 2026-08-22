package log

import (
	"context"
	"errors"
	"io"
	"testing"
)

func attrKeys(attrs []Attr) []string {
	keys := make([]string, 0, len(attrs))
	for _, a := range attrs {
		keys = append(keys, a.Key)
	}
	return keys
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestMergeAttrs pins the precedence rule the whole context-enrichment path
// rests on: what the call site says wins over what the context supplied, and it
// wins in place rather than by appending a second attr under the same key.
// Two attrs sharing a key is not a merge — most encoders emit both, leaving the
// consumer to guess which one is current.
func TestMergeAttrs(t *testing.T) {
	tests := []struct {
		name     string
		base     []Attr
		explicit []Attr
		wantKeys []string
		want     map[string]string
	}{
		{
			name:     "no base returns the explicit attrs",
			explicit: []Attr{String("a", "1")},
			wantKeys: []string{"a"},
			want:     map[string]string{"a": "1"},
		},
		{
			name:     "no explicit attrs returns the base",
			base:     []Attr{String("a", "1")},
			wantKeys: []string{"a"},
			want:     map[string]string{"a": "1"},
		},
		{
			name:     "disjoint keys keep base first",
			base:     []Attr{String("trace", "t1")},
			explicit: []Attr{String("route", "/x")},
			wantKeys: []string{"trace", "route"},
			want:     map[string]string{"trace": "t1", "route": "/x"},
		},
		{
			name:     "explicit overrides in place",
			base:     []Attr{String("trace", "t1"), String("region", "us")},
			explicit: []Attr{String("trace", "t2")},
			wantKeys: []string{"trace", "region"},
			want:     map[string]string{"trace": "t2", "region": "us"},
		},
		{
			name:     "override and append together",
			base:     []Attr{String("trace", "t1")},
			explicit: []Attr{String("trace", "t2"), String("route", "/x")},
			wantKeys: []string{"trace", "route"},
			want:     map[string]string{"trace": "t2", "route": "/x"},
		},
		{
			name:     "both empty",
			wantKeys: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeAttrs(tt.base, tt.explicit)

			if !equalStrings(attrKeys(got), tt.wantKeys) {
				t.Fatalf("keys = %v, want %v", attrKeys(got), tt.wantKeys)
			}
			for _, a := range got {
				if want, ok := tt.want[a.Key]; ok && a.Value.String() != want {
					t.Errorf("attr %q = %q, want %q", a.Key, a.Value.String(), want)
				}
			}
		})
	}
}

// TestMergeAttrsDoesNotMutateInputs pins that merging is non-destructive. The
// base slice is typically returned fresh by a context extractor, but nothing
// stops an extractor from handing back a slice it caches and reuses, and a
// merge that wrote through it would corrupt every later entry.
func TestMergeAttrsDoesNotMutateInputs(t *testing.T) {
	base := []Attr{String("trace", "t1"), String("region", "us")}
	explicit := []Attr{String("trace", "t2")}

	_ = MergeAttrs(base, explicit)

	if base[0].Value.String() != "t1" {
		t.Errorf("base was mutated: trace = %q, want t1", base[0].Value.String())
	}
	if explicit[0].Value.String() != "t2" {
		t.Errorf("explicit was mutated: trace = %q, want t2", explicit[0].Value.String())
	}
}

// TestMapContextAttrExtractor pins the adapter around map-shaped extractors,
// including the stable ordering that makes log output diffable and tests over
// it reproducible — Go map iteration order is randomized per range.
func TestMapContextAttrExtractor(t *testing.T) {
	t.Run("nil extractor stays nil", func(t *testing.T) {
		if MapContextAttrExtractor(nil) != nil {
			t.Fatal("a nil extractor must not be wrapped into a calling one")
		}
	})

	t.Run("empty map yields no attrs", func(t *testing.T) {
		extract := MapContextAttrExtractor(func(context.Context) map[string]any { return nil })
		if got := extract(context.Background()); len(got) != 0 {
			t.Fatalf("got %d attrs, want none", len(got))
		}
	})

	t.Run("keys are sorted", func(t *testing.T) {
		extract := MapContextAttrExtractor(func(context.Context) map[string]any {
			return map[string]any{"zebra": 1, "alpha": 2, "middle": 3}
		})
		want := []string{"alpha", "middle", "zebra"}
		// Repeated because a single pass can match an unsorted implementation by
		// chance.
		for range 10 {
			if got := attrKeys(extract(context.Background())); !equalStrings(got, want) {
				t.Fatalf("keys = %v, want %v", got, want)
			}
		}
	})
}

// TestWrapBackendWithContextMergeRequiresBoth pins that the wrapper degrades to
// the original backend instead of installing one that would dereference nil on
// the first log line.
func TestWrapBackendWithContextMergeRequiresBoth(t *testing.T) {
	inner := &recordingBackend{}

	if got := WrapBackendWithContextMerge(inner, nil); got != Backend(inner) {
		t.Error("a nil extractor must yield the original backend")
	}
	if got := WrapBackendWithContextMerge(nil, func(context.Context) []Attr { return nil }); got != nil {
		t.Error("a nil backend must not be wrapped")
	}
	if got := WrapBackendWithContextMapMerge(inner, nil); got != Backend(inner) {
		t.Error("a nil map extractor must yield the original backend")
	}
}

// TestContextMergeBackendInjectsContextAttrs pins the wrapper end to end: fields
// carried on the context are attached to every entry, and a call site that names
// the same key still wins.
func TestContextMergeBackendInjectsContextAttrs(t *testing.T) {
	inner := &recordingBackend{}
	wrapped := WrapBackendWithContextMapMerge(inner, func(ctx context.Context) map[string]any {
		trace, ok := ctx.Value(ctxKey{}).(string)
		if !ok {
			return nil
		}
		return map[string]any{"trace": trace, "tier": "api"}
	})

	ctx := context.WithValue(context.Background(), ctxKey{}, "t-123")
	wrapped.Log(ctx, LevelInfo, "handled", String("route", "/v1/users"))

	got := inner.last(t)
	values := make(map[string]string, len(got.attrs))
	for _, a := range got.attrs {
		values[a.Key] = a.Value.String()
	}
	if values["trace"] != "t-123" {
		t.Errorf("trace = %q, want t-123", values["trace"])
	}
	if values["tier"] != "api" {
		t.Errorf("tier = %q, want api", values["tier"])
	}
	if values["route"] != "/v1/users" {
		t.Errorf("route = %q, want /v1/users", values["route"])
	}

	// A context carrying nothing must not add empty fields.
	wrapped.Log(context.Background(), LevelInfo, "no context fields", String("route", "/health"))
	if got := inner.last(t); len(got.attrs) != 1 {
		t.Fatalf("got %d attrs without context values, want only the explicit one", len(got.attrs))
	}
}

// TestContextMergeBackendCallSiteWins pins the override direction explicitly:
// a handler that has resolved a better value for a key must not be shadowed by
// the ambient one.
func TestContextMergeBackendCallSiteWins(t *testing.T) {
	inner := &recordingBackend{}
	wrapped := WrapBackendWithContextMapMerge(inner, func(context.Context) map[string]any {
		return map[string]any{"user": "anonymous"}
	})

	wrapped.Log(context.Background(), LevelInfo, "resolved", String("user", "alice"))

	got := inner.last(t)
	if len(got.attrs) != 1 {
		t.Fatalf("got %d attrs, want the key to appear exactly once", len(got.attrs))
	}
	if got.attrs[0].Value.String() != "alice" {
		t.Fatalf("user = %q, want alice", got.attrs[0].Value.String())
	}
}

// TestContextMergeBackendKeepsExtractorOnDerive pins that deriving a logger does
// not quietly drop context enrichment. With is how every named or pre-attributed
// logger in the codebase is built, so losing the extractor there would strip
// trace fields from exactly the components that bothered to name themselves.
func TestContextMergeBackendKeepsExtractorOnDerive(t *testing.T) {
	inner := &recordingBackend{}
	wrapped := WrapBackendWithContextMapMerge(inner, func(context.Context) map[string]any {
		return map[string]any{"tier": "api"}
	})

	derived := wrapped.With(WithName("svc"))
	if inner.withOptions != 1 {
		t.Fatalf("the inner backend received %d options, want 1", inner.withOptions)
	}

	derived.Log(context.Background(), LevelInfo, "still enriched")
	got := inner.last(t)
	if len(got.attrs) != 1 || got.attrs[0].Key != "tier" {
		t.Fatalf("derived backend lost the context extractor: attrs = %v", got.attrs)
	}
}

// TestContextMergeBackendForwardsSync pins that a flush reaches the wrapped
// backend; the wrapper holds no buffer of its own, so swallowing Sync here would
// silently make a pre-exit flush a no-op.
func TestContextMergeBackendForwardsSync(t *testing.T) {
	inner := &recordingBackend{syncErr: context.Canceled}
	wrapped := WrapBackendWithContextMapMerge(inner, func(context.Context) map[string]any { return nil })

	if err := wrapped.Sync(); !errors.Is(err, context.Canceled) {
		t.Fatalf("Sync() = %v, want the wrapped backend's error", err)
	}
}

// TestContextMergeBackendForwardsClose pins that the recommended wrapper does
// not swallow Close. The documented release pattern type-asserts the installed
// backend to io.Closer; a wrapper that does not implement it silently skips the
// release and leaks the file handle underneath.
func TestContextMergeBackendForwardsClose(t *testing.T) {
	inner := &captureBackend{}
	wrapped := WrapBackendWithContextMerge(inner, func(context.Context) []Attr { return nil })

	closer, ok := wrapped.(io.Closer)
	if !ok {
		t.Fatal("context merge backend must implement io.Closer so wrapped handles can be released")
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if !inner.closed {
		t.Fatal("Close() must reach the wrapped backend")
	}
}
