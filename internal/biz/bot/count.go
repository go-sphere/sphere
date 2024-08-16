package bot

import (
	"context"
	"github.com/go-telegram/bot/models"
	"github.com/tbxark/go-base-api/assets/tmpl"
	"math/rand"
)

func (a *App) newCounter(value int) (*Message, error) {
	text, err := tmpl.Execute(a.tmpl.Counter, value)
	if err != nil {
		return nil, err
	}
	return &Message{
		Text: text,
		Button: [][]Button{
			{
				Button{Text: "+", Type: QueryCounter, Data: value + 1},
				Button{Text: "-", Type: QueryCounter, Data: value - 1},
			},
			{
				Button{Text: "Reset", Type: QueryCounter, Data: 0},
				Button{Text: "Random", Type: QueryCounter, Data: rand.Int() % 100},
			},
		},
	}, nil
}

func (a *App) HandleCounter(ctx context.Context, update *models.Update) error {
	value, err := unmarshalUpdateDataX[int](update, 0)
	if err != nil {
		return err
	}
	msg, err := a.newCounter(*value)
	if err != nil {
		return err
	}
	return a.SendMessage(ctx, update, msg)
}
