package telegram

import (
	"log"
	"testing"
)

type testDataStruct struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}

func TestMarshalData(t *testing.T) {
	data := MarshalData[testDataStruct]("test", testDataStruct{
		Number: 123,
		Text:   "456",
	})
	target := `test:[123,"456"]`
	if data != target {
		t.Errorf("Marshaled data is invalid, got: %s", data)
	}
}

func TestUnmarshalData(t *testing.T) {
	raw := `test:[123,"456"]`
	route, data, err := UnmarshalData[testDataStruct](raw)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if data == nil {
		t.Fatal("Unmarshaled data is nil")
	}
	if route != "test" {
		t.Errorf("Unmarshaled route is invalid, got: %s", route)
	}
	if data.Number != 123 {
		t.Errorf("Unmarshaled number is invalid, got: %d", data.Number)
	}
	if data.Text != "456" {
		t.Errorf("Unmarshaled text is invalid, got: %s", data.Text)
	}
	log.Printf("route: %s, data: %v", route, data)
}
