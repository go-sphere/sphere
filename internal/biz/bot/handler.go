package bot

import (
	"context"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"golang.org/x/sync/singleflight"
	"strconv"
)

type HandlerFunc func(ctx context.Context, update *models.Update) error

type HandlerMiddleware func(next HandlerFunc) HandlerFunc

func (h HandlerFunc) WithMiddleware(middleware ...HandlerMiddleware) HandlerFunc {
	handler := h
	for i := len(middleware) - 1; i >= 0; i-- {
		handler = middleware[i](handler)
	}
	return handler
}

func NewSingleFlightMiddleware() HandlerMiddleware {
	sf := &singleflight.Group{}
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, update *models.Update) error {
			if update.CallbackQuery == nil {
				return next(ctx, update)
			}
			key := strconv.Itoa(update.CallbackQuery.Message.Message.ID)
			_, err, _ := sf.Do(key, func() (interface{}, error) {
				return nil, next(ctx, update)
			})
			return err
		}
	}
}

func NewErrorAlertMiddleware(b *bot.Bot) HandlerMiddleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, update *models.Update) error {
			err := next(ctx, update)
			if err != nil {
				if update.Message != nil {
					_, e := b.SendMessage(ctx, &bot.SendMessageParams{
						ChatID: update.Message.Chat.ID,
						Text:   err.Error(),
					})
					return e
				}
				if update.CallbackQuery != nil {
					_, e := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
						CallbackQueryID: update.CallbackQuery.ID,
						Text:            err.Error(),
					})
					return e
				}
			}
			return nil
		}
	}
}
