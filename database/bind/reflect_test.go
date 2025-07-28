package bind

import (
	"reflect"
	"testing"
)

type PublicFields struct {
	GenFileConf
	privateField string
	PublicField  string
}

func Test_getPublicFields(t *testing.T) {
	t.Logf("reflect.VisibleFields")
	fields := reflect.VisibleFields(reflect.TypeFor[PublicFields]())
	for _, field := range fields {
		t.Logf("Field: %s, Index: %v, Anonymous: %v", field.Name, field.Index, field.Anonymous)
	}
	t.Logf("getPublicFields")
	_, fieldMap := getPublicFields(PublicFields{}, func(s string) string {
		return s
	})
	for _, field := range fieldMap {
		t.Logf("Field: %s, Index: %v, Anonymous: %v", field.Name, field.Index, field.Anonymous)
	}
}
