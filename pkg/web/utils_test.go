package web

import (
	"testing"
)

type TestStruct struct {
	Name   string      `json:"name,omitempty"`
	Age    int         `json:"age"`
	Object *TestStruct `json:"object"`
}

func TestConvertObjectToMap(t *testing.T) {
	{
		obj := TestStruct{
			Name: "test",
			Age:  18,
			Object: &TestStruct{
				Name: "sub_test",
				Age:  20,
			},
		}

		result, err := ConvertObjectToMap(&obj)
		if err != nil {
			t.Error(err)
			return
		}
		if result["name"] != "test" {
			t.Error("name not equal")
		}
		if result["age"] != 18 {
			t.Error("age not equal")
		}
		if result["object"].(*TestStruct).Name != "sub_test" {
			t.Error("sub name not equal")
		}
		if result["object"].(*TestStruct).Age != 20 {
			t.Error("sub age not equal")
		}
	}
	{
		result, err := ConvertObjectToMap(nil)
		if err != nil {
			t.Error(err)
			return
		}
		if result != nil {
			t.Error("result not nil")
		}
	}
	{
		_, err := ConvertObjectToMap(1)
		if err == nil {
			t.Error("should return error")
			return
		}
	}
	{
		result, err := ConvertObjectToMap(map[string]string{
			"name": "test",
			"age":  "18",
		})
		if err != nil {
			t.Error(err)
			return
		}
		if result["name"] != "test" {
			t.Error("name not equal")
		}
		if result["age"] != "18" {
			t.Error("age not equal")
		}
	}
}
