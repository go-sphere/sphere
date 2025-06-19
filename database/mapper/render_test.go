package mapper

import (
	"encoding/json"
	"testing"

	"golang.org/x/exp/constraints"
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

func TestPage(t *testing.T) {
	type args[P constraints.Integer] struct {
		total       P
		pageSize    P
		defaultSize P
	}
	type testCase[P constraints.Integer] struct {
		name string
		args args[P]
		want P
	}
	tests := []testCase[int]{
		{
			name: "total is 0",
			args: args[int]{total: 0, pageSize: 10, defaultSize: 20},
			want: 0,
		},
		{
			name: "total is 10, pageSize is 0",
			args: args[int]{total: 10, pageSize: 0, defaultSize: 20},
			want: 1,
		},
		{
			name: "total is 10, pageSize is 20",
			args: args[int]{total: 10, pageSize: 20, defaultSize: 20},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Page(tt.args.total, tt.args.pageSize, tt.args.defaultSize); got != tt.want {
				t.Errorf("Page() = %v, want %v", got, tt.want)
			}
		})
	}
}
