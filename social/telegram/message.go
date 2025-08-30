package telegram

import (
	"bytes"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// Button is an alias for Telegram's inline keyboard button.
type Button = models.InlineKeyboardButton

// NewButton creates an inline keyboard button with text, callback route, and data.
// The route and data are marshaled together to form the callback data.
func NewButton[T any](text, route string, data T) Button {
	return Button{
		Text:         text,
		CallbackData: MarshalData(route, data),
	}
}

// NewURLButton creates an inline keyboard button that opens a URL when pressed.
func NewURLButton(text, url string) Button {
	return Button{
		Text: text,
		URL:  url,
	}
}

// NewBytesInputFile creates an InputFile from a byte slice for file uploads.
// The name parameter specifies the filename that will be used in Telegram.
func NewBytesInputFile(name string, data []byte) models.InputFile {
	return &models.InputFileUpload{
		Filename: name,
		Data:     bytes.NewReader(data),
	}
}

// NewStringInputFile creates an InputFile from a URL string for media sharing.
// This is used when referencing existing media by URL or file ID.
func NewStringInputFile(url string) models.InputFile {
	return &models.InputFileString{
		Data: url,
	}
}

// Message represents a complete message that can be sent or edited in Telegram.
// It supports text content, media attachments, formatting, and inline keyboards.
type Message struct {
	Text      string                          // Message text content
	Media     models.InputFile                // Optional media attachment (photo, document, etc.)
	ParseMode models.ParseMode                // Text parsing mode (HTML, Markdown, etc.)
	Button    [][]models.InlineKeyboardButton // Inline keyboard layout as rows of buttons
}

func (m *Message) toSendMessageParams(chatID int64) *bot.SendMessageParams {
	params := &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      m.Text,
		ParseMode: m.ParseMode,
	}
	if len(m.Button) > 0 {
		params.ReplyMarkup = &models.InlineKeyboardMarkup{
			InlineKeyboard: m.Button,
		}
	}
	return params
}

func (m *Message) toSendPhotoParams(chatID int64) *bot.SendPhotoParams {
	params := &bot.SendPhotoParams{
		ChatID:    chatID,
		Photo:     m.Media,
		Caption:   m.Text,
		ParseMode: m.ParseMode,
	}
	if len(m.Button) > 0 {
		params.ReplyMarkup = &models.InlineKeyboardMarkup{
			InlineKeyboard: m.Button,
		}
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
		params.ReplyMarkup = &models.InlineKeyboardMarkup{
			InlineKeyboard: m.Button,
		}
	}
	return params
}

func (m *Message) toEditMessageCaptionParams(chatID int64, messageID int) *bot.EditMessageCaptionParams {
	params := &bot.EditMessageCaptionParams{
		ChatID:    chatID,
		MessageID: messageID,
		Caption:   m.Text,
		ParseMode: m.ParseMode,
	}
	if len(m.Button) > 0 {
		params.ReplyMarkup = &models.InlineKeyboardMarkup{
			InlineKeyboard: m.Button,
		}
	}
	return params
}

func (m *Message) toEditMessageMediaParams(chatID int64, messageID int) *bot.EditMessageMediaParams {
	params := &bot.EditMessageMediaParams{
		ChatID:    chatID,
		MessageID: messageID,
		Media:     nil,
	}
	photo := &models.InputMediaPhoto{
		Caption:   m.Text,
		ParseMode: m.ParseMode,
	}
	if upload, ok := m.Media.(*models.InputFileUpload); ok {
		photo.Media = "attach://" + upload.Filename
		photo.MediaAttachment = upload.Data
	}
	if url, ok := m.Media.(*models.InputFileString); ok {
		photo.Media = url.Data
	}
	params.Media = photo
	if len(m.Button) > 0 {
		params.ReplyMarkup = &models.InlineKeyboardMarkup{
			InlineKeyboard: m.Button,
		}
	}
	return params
}
