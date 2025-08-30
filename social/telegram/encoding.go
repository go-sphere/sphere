package telegram

import (
	"fmt"
	"strings"

	"github.com/TBXark/jsoncompressor"
)

// query prefix must be unique and has suffix ":" to separate the data
// update.CallbackQuery.Data format: $route:json($data)

// UnmarshalData decodes Telegram callback query data into a route and typed data structure.
// The input data should be formatted as "route:compressed_json_data".
// Returns the route string, the unmarshaled data of type T, and any error encountered.
// This is commonly used for handling Telegram bot callback queries with structured data.
func UnmarshalData[T any](data string) (string, *T, error) {
	cmp := strings.SplitN(data, ":", 2)
	if len(cmp) != 2 {
		return "", nil, fmt.Errorf("invalid data format")
	}
	var v T
	err := jsoncompressor.Unmarshal([]byte(cmp[1]), &v)
	if err != nil {
		return cmp[0], nil, err
	}
	return cmp[0], &v, nil
}

// MarshalData encodes a route and typed data into Telegram callback query format.
// The data is compressed using JSON compression and formatted as "route:compressed_json_data".
// Returns the formatted string suitable for use in Telegram callback queries.
func MarshalData[T any](route string, data T) string {
	b, _ := jsoncompressor.Marshal(data)
	return fmt.Sprintf("%s:%s", route, string(b))
}
