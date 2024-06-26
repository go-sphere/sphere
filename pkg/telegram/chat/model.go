package chat

import (
	"encoding/json"
)

type RPCData[T any] struct {
	Type string `json:"t"`
	Data T      `json:"d"`
}

func EmptyRPCData(t string) string {
	j := struct {
		Type string `json:"t"`
	}{
		Type: t,
	}
	bytes, err := json.Marshal(j)
	if err != nil {
		return ""
	}
	return string(bytes)
}

func NewRPCData[T any](t string, data T) string {
	j := RPCData[T]{
		Type: t,
		Data: data,
	}
	bytes, err := json.Marshal(j)
	if err != nil {
		return ""
	}
	return string(bytes)
}

func DecodeRPCData[T any](data string) (*T, error) {
	var v *T
	err := json.Unmarshal([]byte(data), v)
	if err != nil {
		return nil, err
	}
	return v, nil
}

type RPCButton struct {
	Text string
	Data string
}

func RPCTextButton(title string, page string) RPCButton {
	return RPCButton{
		Text: title,
		Data: EmptyRPCData(page),
	}
}

func RPCDataButton[T any](title string, page string, data T) RPCButton {
	return RPCButton{
		Text: title,
		Data: NewRPCData(page, data),
	}
}

func RPCMultiDataButton[T any](title string, page string, data ...T) RPCButton {
	return RPCButton{
		Text: title,
		Data: NewRPCData(page, data),
	}
}

func RPCRow(buttons ...RPCButton) []RPCButton {
	var row []RPCButton
	row = append(row, buttons...)
	return row
}

func RPCSections(rows ...[]RPCButton) [][]RPCButton {
	var keyboard [][]RPCButton
	keyboard = append(keyboard, rows...)
	return keyboard
}

type Message struct {
	Text     string
	Menu     string
	Sections [][]RPCButton
}
