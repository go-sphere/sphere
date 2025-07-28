package bind

import (
	"reflect"
	"testing"
)

type Test struct {
	GenFileConf
	privateField string
	PublicField  string
}

func (t Test) privateMethod() {
}

func (t Test) PublicMethod() {
}

func (t *Test) privateMethodPtr() {
}

func (t *Test) PublicMethodPtr() {
}

func Test_getPublicFields(t *testing.T) {
	t.Logf("reflect.VisibleFields")
	fields := reflect.VisibleFields(reflect.TypeFor[Test]())
	for _, field := range fields {
		t.Logf("Field: %s, Index: %v, Anonymous: %v", field.Name, field.Index, field.Anonymous)
	}
	t.Logf("getPublicFields")
	_, fieldMap := getPublicFields(Test{}, func(s string) string {
		return s
	})
	for _, field := range fieldMap {
		t.Logf("Field: %s, Index: %v, Anonymous: %v", field.Name, field.Index, field.Anonymous)
	}
}

type MyString string

func (s MyString) String() string {
	return string(s)
}

func (s *MyString) StringPtr() string {
	return string(*s)
}

func Test_getPublicMethods(t *testing.T) {
	methods, _ := getPublicMethods(Test{}, func(s string) string {
		return s
	})
	for _, method := range methods {
		t.Logf("Method: %s", method)
	}
}

func Test_getPublicMethodsWithString(t *testing.T) {
	methods, _ := getPublicMethods(MyString("test"), func(s string) string {
		return s
	})
	for _, method := range methods {
		t.Logf("Method: %s", method)
	}
}
