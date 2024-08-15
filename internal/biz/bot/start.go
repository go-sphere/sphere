package bot

import (
	"context"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/tbxark/go-base-api/pkg/log"
)

func (a *App) HandleStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Hello, I'm a bot",
	})
	if err != nil {
		log.Errorf("send message error: %v", err)
	}
}
