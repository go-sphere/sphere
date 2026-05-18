package redis

import (
	"context"
	"slices"
	"testing"

	"github.com/go-sphere/sphere/test/redistest"
)

func newTestCache(t *testing.T) *ByteCache {
	t.Helper()
	c := NewByteCache(redistest.NewTestRedisClient(t))
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestKeys(t *testing.T) {
	ctx := context.Background()
	c := newTestCache(t)

	if err := c.MultiSet(ctx, map[string][]byte{
		"a:1": []byte("1"),
		"a:2": []byte("2"),
		"b:1": []byte("3"),
	}); err != nil {
		t.Fatalf("MultiSet: %v", err)
	}

	got, err := c.Keys(ctx, "a:")
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	slices.Sort(got)
	want := []string{"a:1", "a:2"}
	if !slices.Equal(got, want) {
		t.Fatalf("Keys mismatch: got=%v want=%v", got, want)
	}
}

func TestKeysGlobEscaping(t *testing.T) {
	ctx := context.Background()
	c := newTestCache(t)

	if err := c.MultiSet(ctx, map[string][]byte{
		"weird*ns:1": []byte("1"),
		"weird*ns:2": []byte("2"),
		"weirdXns:3": []byte("3"),
	}); err != nil {
		t.Fatalf("MultiSet: %v", err)
	}

	got, err := c.Keys(ctx, "weird*ns:")
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	slices.Sort(got)
	want := []string{"weird*ns:1", "weird*ns:2"}
	if !slices.Equal(got, want) {
		t.Fatalf("Keys mismatch: got=%v want=%v (weirdXns:3 must not match)", got, want)
	}
}

func TestEscapeGlob(t *testing.T) {
	cases := map[string]string{
		"":         "",
		"plain":    "plain",
		"a*b":      `a\*b`,
		"a?b":      `a\?b`,
		"a[bc]":    `a\[bc\]`,
		`a\b`:      `a\\b`,
		"users:":   "users:",
		"weird*ns": `weird\*ns`,
	}
	for in, want := range cases {
		if got := escapeGlob(in); got != want {
			t.Errorf("escapeGlob(%q) = %q, want %q", in, got, want)
		}
	}
}
