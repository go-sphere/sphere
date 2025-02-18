package gen

import (
	"log"
	"testing"
)

func TestGenerate(t *testing.T) {
	jsonStr := `
	{
		"userId": "123456",
		"userName": "John Doe",
		"age": 30,
		"isActive": true,
		"score": 95.5,
		"address": {
			"street": "123 Main St",
			"city": "New York",
			"zipCode": "10001"
		},
		"tags": ["user", "active", "premium"],
		"preferences": [
			{
				"theme": "dark",
				"notifications": true
			}
		]
	}`
	text, err := Generate("config", "config.proto", "Config", []byte(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	log.Printf("Generated proto:\n%s", text)
}
