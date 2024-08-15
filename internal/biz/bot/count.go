package bot

import (
	"context"
	"fmt"
	"github.com/go-telegram/bot/models"
	"math/rand"
)

func (a *App) newCounter(value int) *Message {
	return &Message{
		Text: fmt.Sprintf("Current value: %d", value),
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
	}
}

func (a *App) HandleCounter(ctx context.Context, update *models.Update) error {
	value, err := unmarshalUpdateDataX[int](update, 0)
	if err != nil {
		return err
	}
	msg := a.newCounter(*value)
	return a.SendMessage(ctx, update, msg)
}
