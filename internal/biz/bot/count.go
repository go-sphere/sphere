package bot

import (
	"context"
	"fmt"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/tbxark/go-base-api/pkg/log"
	"math/rand"
)

func (a *App) HandleCounter(ctx context.Context, b *bot.Bot, update *models.Update) {
	value := 0
	if update.CallbackQuery != nil {
		if v, err := unmarshalData[int](update.CallbackQuery.Data); err == nil {
			value = *v
		}
	}
	msg := Message{
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
	_, err := SendMenuMessage(ctx, &msg, b, update)
	if err != nil {
		log.Errorf("send message error: %v", err)
	}
}
