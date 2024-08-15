package bot

import (
	"encoding/json"
	"fmt"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
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

type MenuButton struct {
	Text string
	Type string
	Data any
}

type MenuMessage struct {
	Text      string
	Button    [][]MenuButton
	ParseMode models.ParseMode
}

func (m *MenuMessage) toInlineKeyboardMarkup() *models.InlineKeyboardMarkup {
	if len(m.Button) == 0 {
		return nil
	}
	keyboard := make([][]models.InlineKeyboardButton, 0, len(m.Button))
	for _, row := range m.Button {
		buttons := make([]models.InlineKeyboardButton, 0, len(row))
		for _, btn := range row {
			buttons = append(buttons, models.InlineKeyboardButton{
				Text:         btn.Text,
				CallbackData: marshalData(btn.Type, btn.Data),
			})
		}
		keyboard = append(keyboard, buttons)
	}
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: keyboard,
	}
}

func (m *MenuMessage) toSendMessageParams(chatID int64) *bot.SendMessageParams {
	return &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        m.Text,
		ParseMode:   m.ParseMode,
		ReplyMarkup: m.toInlineKeyboardMarkup(),
	}
}

func (m *MenuMessage) toEditMessageTextParams(chatID int64, messageID int) *bot.EditMessageTextParams {
	return &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        m.Text,
		ParseMode:   m.ParseMode,
		ReplyMarkup: m.toInlineKeyboardMarkup(),
	}
}
