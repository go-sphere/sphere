package mapper

import (
	"encoding/json"
	"testing"
)

func TestMapStruct(t *testing.T) {
	type structA struct {
		Name     string `json:"name"`
		Age      int    `json:"age"`
		Raw      []byte `json:"raw"`
		Internal struct {
			Name string `json:"name"`
		} `json:"internal"`
	}

	type structB struct {
		Name     *string `json:"name"`
		Age      int     `json:"age"`
		Raw      []byte  `json:"raw"`
		Internal *struct {
			Name string `json:"name"`
		} `json:"internal"`
	}

	a := structA{
		Name: "Alice",
		Age:  25,
		Raw:  []byte("raw"),
		Internal: struct {
			Name string `json:"name"`
		}{
			Name: "InternalName",
		},
	}
	b := MapStruct[structA, structB](&a)
	if b == nil {
		t.Errorf("MapStruct() error = %v", b)
		return
	}
	if *b.Name != a.Name {
		t.Errorf("MapStruct() = %v, want %v", *b.Name, a.Name)
	}
	if b.Age != a.Age {
		t.Errorf("MapStruct() = %v, want %v", b.Age, a.Age)
	}
	if string(b.Raw) != string(a.Raw) {
		t.Errorf("MapStruct() = %v, want %v", string(b.Raw), string(a.Raw))
	}
	if b.Internal == nil {
		t.Errorf("MapStruct() = %v, want %v", b.Internal, a.Internal)
	}
	if b.Internal.Name != a.Internal.Name {
		t.Errorf("MapStruct() = %v, want %v", b.Internal.Name, a.Internal.Name)
	}
	bytes, err := json.Marshal(b)
	if err != nil {
		t.Errorf("MapStruct() error = %v", err)
		return
	}
	t.Logf("MapStruct() = %s", bytes)
}
