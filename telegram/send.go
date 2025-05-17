package telegram

import (
	"context"
	"github.com/go-telegram/bot"
)

func SendMessage(ctx context.Context, b *bot.Bot, update *Update, m *Message) error {
	if m == nil || update == nil {
		return nil
	}
	if update.CallbackQuery != nil {
		origin := update.CallbackQuery.Message.Message
		if len(origin.Photo) == 0 {
			param := m.toEditMessageTextParams(origin.Chat.ID, origin.ID)
			_, err := b.EditMessageText(ctx, param)
			return err
		} else {
			if m.Media == nil {
				param := m.toEditMessageCaptionParams(origin.Chat.ID, origin.ID)
				_, err := b.EditMessageCaption(ctx, param)
				return err
			} else {
				param := m.toEditMessageMediaParams(origin.Chat.ID, origin.ID)
				_, err := b.EditMessageMedia(ctx, param)
				return err
			}
		}
	}
	if update.Message != nil {
		if m.Media == nil {
			param := m.toSendMessageParams(update.Message.Chat.ID)
			_, err := b.SendMessage(ctx, param)
			return err
		} else {
			param := m.toSendPhotoParams(update.Message.Chat.ID)
			_, err := b.SendPhoto(ctx, param)
			return err
		}
	}
	return nil
}

func SendErrorMessage(ctx context.Context, b *bot.Bot, update *Update, err error) {
	if err == nil {
		return
	}
	if update.Message != nil {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   err.Error(),
		})
	}
	if update.CallbackQuery != nil {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            err.Error(),
		})
	}
}
