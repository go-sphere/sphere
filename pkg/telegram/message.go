package telegram

import (
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Button struct {
	Text string
	Type string
	Data any
}

type Message struct {
	Text      string
	Button    [][]Button
	ParseMode models.ParseMode
}

func (m *Message) toInlineKeyboardMarkup() *models.InlineKeyboardMarkup {
	keyboard := make([][]models.InlineKeyboardButton, 0, len(m.Button))
	for _, row := range m.Button {
		buttons := make([]models.InlineKeyboardButton, 0, len(row))
		for _, btn := range row {
			buttons = append(buttons, models.InlineKeyboardButton{
				Text:         btn.Text,
				CallbackData: MarshalData(btn.Type, btn.Data),
			})
		}
		keyboard = append(keyboard, buttons)
	}
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: keyboard,
	}
}

func (m *Message) toSendMessageParams(chatID int64) *bot.SendMessageParams {
	params := &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      m.Text,
		ParseMode: m.ParseMode,
	}
	if len(m.Button) > 0 {
		params.ReplyMarkup = m.toInlineKeyboardMarkup()
	}
	return params
}

func (m *Message) toEditMessageTextParams(chatID int64, messageID int) *bot.EditMessageTextParams {
	params := &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      m.Text,
		ParseMode: m.ParseMode,
	}
	if len(m.Button) > 0 {
		params.ReplyMarkup = m.toInlineKeyboardMarkup()
	}
	return params
}
