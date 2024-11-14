package tuple

import "testing"

func TestSetToStruct(t *testing.T) {
	var test struct {
		Key   string
		Value int
	}
	tuple := New2("hello", 42)
	err := SetToStruct(&tuple, &test, 0)
	if err != nil {
		t.Fatal(err)
	}
	if test.Key != tuple.First {
		t.Fatalf("expected %q, got %q", tuple.First, test.Key)
	}
	if test.Value != tuple.Second {
		t.Fatalf("expected %d, got %d", tuple.Second, test.Value)
	}

	var test2 struct {
		hidden string
		Key    string
		Value  int
	}
	err = SetToStruct(&tuple, &test2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if test2.Key != tuple.First {
		t.Fatalf("expected %q, got %q", tuple.First, test2.Key)
	}
	if test2.Value != tuple.Second {
		t.Fatalf("expected %d, got %d", tuple.Second, test2.Value)
	}

}
