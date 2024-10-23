package auth

import (
	"reflect"
	"testing"
)

func TestAuth_strParser(t *testing.T) {
	{
		parser := strParser(reflect.TypeOf(uint8(0)).Kind())
		id, err := parser("1")
		if err != nil {
			t.Error(err)
			return
		}
		if i, ok := id.(uint8); !ok || i != 1 {
			t.Error("parse error")
		}
		if _, e := parser("1234567890"); e != nil {
			t.Log(e)
		} else {
			t.Error("expect error")
		}
	}
	{
		parser := strParser(reflect.TypeOf(int64(0)).Kind())
		id, err := parser("123456789")
		if err != nil {
			t.Error(err)
			return
		}
		if i, ok := id.(int64); !ok || i != 123456789 {
			t.Error("parse error")
		}
		if _, ok := id.(uint64); ok {
			t.Error("parse error")
		}
	}
}
