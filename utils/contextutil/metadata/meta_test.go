package metadata

import (
	"context"
	"reflect"
	"testing"
)

func TestMeta(t *testing.T) {
	testData := map[string]any{
		"key1": "value1",
		"key2": 123,
	}

	// Test case 1: WithMeta and MetaFrom
	ctx := WithMeta(context.Background(), testData)
	meta := MetaFrom(ctx)

	if !reflect.DeepEqual(meta, testData) {
		t.Errorf("MetaFrom() = %v, want %v", meta, testData)
	}

	// Test case 2: MetaFrom with no meta
	ctx = context.Background()
	meta = MetaFrom(ctx)
	if meta != nil {
		t.Errorf("MetaFrom() from context with no meta should be nil, but got %v", meta)
	}

	// Test case 3: MetaFrom with wrong type
	ctx = context.WithValue(context.Background(), metaContextKey, "not a map")
	meta = MetaFrom(ctx)
	if meta != nil {
		t.Errorf("MetaFrom() from context with wrong type should be nil, but got %v", meta)
	}
}

func TestWithMetaCopiesInput(t *testing.T) {
	original := map[string]any{
		"key1": "value1",
	}

	ctx := WithMeta(context.Background(), original)

	// Mutating the original map after WithMeta must not affect the stored copy.
	original["key1"] = "mutated"
	original["key2"] = "added"

	meta := MetaFrom(ctx)
	if meta["key1"] != "value1" {
		t.Errorf("stored meta key1 = %v, want %q (input mutation leaked into context)", meta["key1"], "value1")
	}
	if _, ok := meta["key2"]; ok {
		t.Errorf("stored meta unexpectedly gained key2 from input mutation")
	}
}
