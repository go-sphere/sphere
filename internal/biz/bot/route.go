package bot

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	CommandStart   = "/start"
	CommandCounter = "/counter"
)

// query prefix must be unique and has suffix ":" to separate the data
// update.CallbackQuery.Data format: $prefix:$data

const (
	QueryCounter = "counter:"
)

func unmarshalData[T any](data string) (*T, error) {
	cmp := strings.SplitN(data, ":", 2)
	if len(cmp) != 2 {
		return nil, fmt.Errorf("invalid data format")
	}
	var v T
	err := json.Unmarshal([]byte(cmp[1]), &v)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func marshalData[T any](t string, data T) string {
	b, _ := json.Marshal(data)
	return fmt.Sprintf("%s%s", t, string(b))
}
