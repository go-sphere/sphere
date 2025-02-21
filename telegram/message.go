package telegram

import (
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Button struct {
	Text         string
	Type         string
	CallbackData string
}

func NewButton[T any](text, dType string, data T) Button {
	return Button{
		Text:         text,
		Type:         dType,
		CallbackData: MarshalData(dType, data),
	}
}

type Message struct {
	Text      string
	Media     models.InputFile
	ParseMode models.ParseMode
	Button    [][]Button
}

func (m *Message) toInlineKeyboardMarkup() *models.InlineKeyboardMarkup {
	keyboard := make([][]models.InlineKeyboardButton, 0, len(m.Button))
	for _, row := range m.Button {
		buttons := make([]models.InlineKeyboardButton, 0, len(row))
		for _, btn := range row {
			buttons = append(buttons, models.InlineKeyboardButton{
				Text:         btn.Text,
				CallbackData: btn.CallbackData,
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

func (m *Message) toSendPhotoParams(chatID int64) *bot.SendPhotoParams {
	params := &bot.SendPhotoParams{
		ChatID:    chatID,
		Caption:   m.Text,
		ParseMode: m.ParseMode,
		Photo:     m.Media,
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

func (m *Message) toEditMessageCaptionParams(chatID int64, messageID int) *bot.EditMessageCaptionParams {
	params := &bot.EditMessageCaptionParams{
		ChatID:    chatID,
		MessageID: messageID,
		Caption:   m.Text,
	}
	if len(m.Button) > 0 {
		params.ReplyMarkup = m.toInlineKeyboardMarkup()
	}
	return params
}

func (m *Message) toEditMessageMediaParams(chatID int64, messageID int) *bot.EditMessageMediaParams {
	params := &bot.EditMessageMediaParams{
		ChatID:    chatID,
		MessageID: messageID,
		Media: &models.InputMediaPhoto{
			Caption:   m.Text,
			ParseMode: m.ParseMode,
		},
	}
	photo := &models.InputMediaPhoto{}
	if upload, ok := m.Media.(*models.InputFileUpload); ok {
		photo.Media = "attach://" + upload.Filename
		photo.MediaAttachment = upload.Data
	}
	if url, ok := m.Media.(*models.InputFileString); ok {
		photo.Media = url.Data
	}
	params.Media = photo
	if len(m.Button) > 0 {
		params.ReplyMarkup = m.toInlineKeyboardMarkup()
	}
	return params
}
