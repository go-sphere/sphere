package bot

import (
	"context"
	"github.com/go-telegram/bot/models"
	"github.com/tbxark/go-base-api/pkg/telegram"
	"math/rand"
)

func UnmarshalUpdateDataWithDefault[T any](update *models.Update, defaultValue T) T {
	if update != nil && update.CallbackQuery != nil {
		_, data, err := telegram.UnmarshalData[T](update.CallbackQuery.Data)
		if err == nil {
			return *data
		}
		return defaultValue
	} else {
		return defaultValue
	}
}

func (b *Bot) newCounter(value int) (*telegram.Message, error) {
	text, err := renderText(counterTemplate, value)
	if err != nil {
		return nil, err
	}
	return &telegram.Message{
		Text: text,
		Button: [][]telegram.Button{
			{
				telegram.Button{Text: "+", Type: QueryCounter, Data: value + 1},
				telegram.Button{Text: "-", Type: QueryCounter, Data: value - 1},
			},
			{
				telegram.Button{Text: "Reset", Type: QueryCounter, Data: 0},
				telegram.Button{Text: "Random", Type: QueryCounter, Data: rand.Int() % 100},
			},
		},
	}, nil
}

func (b *Bot) HandleCounter(ctx context.Context, update *models.Update) error {
	value := UnmarshalUpdateDataWithDefault(update, 0)
	msg, err := b.newCounter(value)
	if err != nil {
		return err
	}
	return b.SendMessage(ctx, update, msg)
}

func (b *Bot) HandleStart(ctx context.Context, update *models.Update) error {
	text, err := renderText(startTemplate, update.Message.From.FirstName)
	if err != nil {
		return err
	}
	return b.SendMessage(ctx, update, &telegram.Message{
		Text: text,
	})
}
