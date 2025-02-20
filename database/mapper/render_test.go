package mapper

import (
	"reflect"
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

func TestSetFields(t *testing.T) {
	names := []string{"Alice", "", "Charlie"}
	tests := []struct {
		name       string
		source     Source
		ignoreZero bool
		want       Target
	}{
		{
			name: "Basic Test",
			source: Source{
				Name:     names[0],
				Age:      25,
				IsActive: true,
			},
			ignoreZero: false,
			want: Target{
				name:     &names[0],
				age:      25,
				isActive: true,
			},
		},
		{
			name: "Ignore zero value test",
			source: Source{
				Name:     names[1],
				Age:      0,
				IsActive: false,
			},
			ignoreZero: true,
			want: Target{
				name:     nil,
				age:      0,
				isActive: false,
			},
		},
		{
			name: "Partial field test",
			source: Source{
				Name:     names[2],
				Age:      0,
				IsActive: true,
			},
			ignoreZero: true,
			want: Target{
				name:     &names[2],
				age:      0,
				isActive: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := &Target{}
			err := SetFields(tt.source, target, tt.ignoreZero)
			if err != nil {
				t.Errorf("SetFields() error = %v", err)
				return
			}

			if !reflect.DeepEqual(*target, tt.want) {
				t.Errorf("SetFields() = %v, want %v", *target, tt.want)
			}
		})
	}
}

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
