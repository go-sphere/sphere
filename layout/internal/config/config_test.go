package config

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestConfig(t *testing.T) {
	cfg := NewEmptyConfig()
	elem := reflect.ValueOf(cfg).Elem()
	fields := make([]string, 0)
	for i := 0; i < elem.NumField(); i++ {
		fields = append(fields, fmt.Sprintf("\"%s\"", elem.Type().Field(i).Name))
	}
	fmt.Printf("\twire.FieldsOf(new(*Config), %s),\n", strings.Join(fields, ", "))
}
