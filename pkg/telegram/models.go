package telegram

type MethodExtraData struct {
	Command       string
	CallbackQuery string
}

func NewMethodExtraData(raw map[string]string) MethodExtraData {
	return MethodExtraData{
		Command:       raw["command"],
		CallbackQuery: raw["callback_query"],
	}
}
