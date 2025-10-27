package bind

import (
	"reflect"
	"testing"
)

type Test struct {
	GenFileConf
	privateField string //nolint
	PublicField  string
}

func (t Test) privateMethod() { //nolint
}

func (t Test) PublicMethod() { //nolint
}

func (t *Test) privateMethodPtr() { //nolint
}

func (t *Test) PublicMethodPtr() { //nolint
}

func Test_extractPublicFields(t *testing.T) {
	type args struct {
		obj       interface{}
		keyMapper func(s string) string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "test",
			args: args{
				obj: Test{},
				keyMapper: func(s string) string {
					return s
				},
			},
			want: []string{
				"PublicField",
			},
		},
		{
			name: "test_pointer",
			args: args{
				obj: &Test{},
				keyMapper: func(s string) string {
					return s
				},
			},
			want: []string{
				"PublicField",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := extractPublicFields(tt.args.obj, tt.args.keyMapper)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractPublicFields() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_extractPublicMethods(t *testing.T) {
	type args struct {
		obj       any
		keyMapper func(string) string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "test",
			args: args{
				obj: Test{},
				keyMapper: func(s string) string {
					return s
				},
			},
			want: []string{
				"PublicMethod",
				"PublicMethodPtr",
			},
		},
		{
			name: "test_pointer",
			args: args{
				obj: &Test{},
				keyMapper: func(s string) string {
					return s
				},
			},
			want: []string{
				"PublicMethod",
				"PublicMethodPtr",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := extractPublicMethods(tt.args.obj, tt.args.keyMapper)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractPublicMethods() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_extractPackageImport(t *testing.T) {
	type args struct {
		val any
	}
	tests := []struct {
		name string
		args args
		want [2]string
	}{
		{
			name: "test",
			args: args{
				val: Test{},
			},
			want: [2]string{
				"github.com/go-sphere/sphere/database/bind",
				"bind",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractPackageImport(tt.args.val); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractPackageImport() = %v, want %v", got, tt.want)
			}
		})
	}
}
