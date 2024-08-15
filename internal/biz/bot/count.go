package bot

import (
	"context"
	"fmt"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"math/rand"
)

func (a *App) HandleCounter(ctx context.Context, b *bot.Bot, update *models.Update) {
	value := 0
	if update.CallbackQuery != nil {
		if v, err := unmarshalData[int](update.CallbackQuery.Data); err == nil {
			value = *v
		}
	}
	msg := MenuMessage{
		Text: fmt.Sprintf("Current value: %d", value),
		Button: [][]MenuButton{
			{
				MenuButton{Text: "+", Type: QueryCounter, Data: value + 1},
				MenuButton{Text: "-", Type: QueryCounter, Data: value - 1},
			},
			{
				MenuButton{Text: "Reset", Type: QueryCounter, Data: 0},
				MenuButton{Text: "Random", Type: QueryCounter, Data: rand.Int() % 100},
			},
		},
	}

	if update.CallbackQuery != nil {
		origin := update.CallbackQuery.Message.Message
		_, _ = b.EditMessageText(ctx, msg.toEditMessageTextParams(origin.Chat.ID, origin.ID))
	} else {
		_, _ = b.SendMessage(ctx, msg.toSendMessageParams(update.Message.Chat.ID))
	}
}
