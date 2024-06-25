package chat

import (
	"encoding/json"
)

type RPCData[T any] struct {
	Type string `json:"t"`
	Data T      `json:"d"`
}

type RPCButton struct {
	Text string
	Desc string
	Data string
}

type RPCSection struct {
	//Title    string
	Sections [][]RPCButton
}

func CreateEmptyRPCData(t string) string {
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

func CreateRPCData[T any](t string, data T) string {
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

func RPCDataButton[T any](title string, page string, data T) RPCButton {
	return RPCButton{
		Text: title,
		Data: CreateRPCData(page, data),
	}
}

func RPCDataListButton[T any](title string, page string, data ...T) RPCButton {
	return RPCButton{
		Text: title,
		Data: CreateRPCData(page, data),
	}
}

func RPCTextButton(title string, page string) RPCButton {
	return RPCButton{
		Text: title,
		Data: CreateEmptyRPCData(page),
	}
}

func RPCRows(buttons ...RPCButton) []RPCButton {
	var row []RPCButton
	row = append(row, buttons...)
	return row
}

func RPCButtons(rows ...[]RPCButton) [][]RPCButton {
	var keyboard [][]RPCButton
	keyboard = append(keyboard, rows...)
	return keyboard
}

type Message struct {
	Text     string
	Menu     string
	Sections []RPCSection
}
