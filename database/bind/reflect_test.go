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

func Test_ExtractPublicFields(t *testing.T) {
	t.Logf("reflect.VisibleFields")
	fields := reflect.VisibleFields(reflect.TypeFor[Test]())
	for _, field := range fields {
		t.Logf("Field: %s, Index: %v, Anonymous: %v", field.Name, field.Index, field.Anonymous)
	}
	t.Logf("extractPublicFields")
	_, fieldMap := extractPublicFields(Test{}, func(s string) string {
		return s
	})
	for _, field := range fieldMap {
		t.Logf("Field: %s, Index: %v, Anonymous: %v", field.Name, field.Index, field.Anonymous)
	}
}

type MyString string

func (s MyString) String() string { //nolint
	return string(s)
}

func (s *MyString) StringPtr() string { //nolint
	return string(*s)
}

func Test_ExtractPublicMethods(t *testing.T) {
	methods, _ := extractPublicMethods(Test{}, func(s string) string {
		return s
	})
	for _, method := range methods {
		t.Logf("Method: %s", method)
	}
}

func Test_ExtractPublicMethodsWithString(t *testing.T) {
	methods, _ := extractPublicMethods(MyString("test"), func(s string) string {
		return s
	})
	for _, method := range methods {
		t.Logf("Method: %s", method)
	}
}

func TestExtractPackageImport(t *testing.T) {
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
