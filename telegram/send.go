package telegram

import (
	"context"
	"errors"
	"github.com/go-telegram/bot"
	"golang.org/x/time/rate"
	"time"
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

func BroadcastMessage[T any](ctx context.Context, b *bot.Bot, data []T, rateLimiter *rate.Limiter, send func(context.Context, *bot.Bot, T) error, progress func(int, int)) error {
	total := len(data)
	for i, d := range data {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if progress != nil {
			progress(i, total)
		}
		err := rateLimiter.Wait(ctx)
		if err != nil {
			return err
		}
		err = send(ctx, b, d)
		if err != nil {
			return err
		}
	}
	if progress != nil {
		progress(total, total)
	}
	return nil
}

func RetryOnTooManyRequestsError(maxRetries int, send func() error) error {
	if maxRetries < 0 {
		return errors.New("max retries exceeded")
	}
	err := send()
	if err == nil {
		return nil
	}
	var tooManyRequestsError *bot.TooManyRequestsError
	if errors.As(err, &tooManyRequestsError) {
		sleepDuration := time.Duration(tooManyRequestsError.RetryAfter) * time.Second
		time.Sleep(sleepDuration)
		return RetryOnTooManyRequestsError(maxRetries-1, send)
	}
	return err
}
