package telegram

import "github.com/go-telegram/bot/models"

// Update is an alias for the Telegram bot library's Update type.
// It represents an incoming update from the Telegram Bot API.
type Update = models.Update

// MethodExtraData holds additional routing information extracted from Telegram updates.
// It provides convenient access to command and callback query data for request handling.
type MethodExtraData struct {
	Command       string // The command extracted from the update (e.g., "/start")
	CallbackQuery string // The callback query data from inline keyboard interactions
}

// NewMethodExtraData creates a new MethodExtraData instance from a raw string map.
// This is typically used during request processing to extract routing information.
func NewMethodExtraData(raw map[string]string) *MethodExtraData {
	return &MethodExtraData{
		Command:       raw["command"],
		CallbackQuery: raw["callback_query"],
	}
}
