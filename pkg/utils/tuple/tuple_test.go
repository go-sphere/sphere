package tuple

import (
	"testing"
)

//func Test_GenTuple(t *testing.T) {
//	gen.Gen(os.Stdout)
//}

func TestOf2_UnmarshalJSON(t *testing.T) {
	pair := New2("hello", 42)
	data, err := pair.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var pair2 Of2[string, int]
	err = pair2.UnmarshalJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if pair.First != pair2.First {
		t.Fatalf("expected %q, got %q", pair.First, pair2.First)
	}
	if pair.Second != pair2.Second {
		t.Fatalf("expected %d, got %d", pair.Second, pair2.Second)
	}
}

func TestOf3_UnmarshalJSON(t *testing.T) {
	pair := New3("hello", 42, "world")
	data, err := pair.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var pair2 Of3[string, int, string]
	err = pair2.UnmarshalJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if pair.First != pair2.First {
		t.Fatalf("expected %q, got %q", pair.First, pair2.First)
	}
	if pair.Second != pair2.Second {
		t.Fatalf("expected %d, got %d", pair.Second, pair2.Second)
	}
	if pair.Third != pair2.Third {
		t.Fatalf("expected %q, got %q", pair.Third, pair2.Third)
	}
}
