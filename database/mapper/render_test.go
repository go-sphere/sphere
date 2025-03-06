package mapper

import (
	"testing"
)

type Source struct {
	Name     string
	Age      int
	IsActive bool
}

type Target struct {
	name     *string
	age      int
	isActive bool
}

func (t *Target) SetName(name string)       { t.name = &name }
func (t *Target) SetAge(age int64)          { t.age = int(age) }
func (t *Target) SetIsActive(isActive bool) { t.isActive = isActive }

func TestMapStruct(t *testing.T) {
	type structA struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
		Raw  []byte `json:"raw"`
	}

	type structB struct {
		Name *string `json:"name"`
		Age  int     `json:"age"`
		Raw  []byte  `json:"raw"`
	}

	a := structA{
		Name: "Alice",
		Age:  25,
		Raw:  []byte("raw"),
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
	t.Logf("MapStruct() = %+v", *b)
}
