package bot

import (
	"context"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (a *App) HandleStart(ctx context.Context, update *models.Update) error {
	_, err := a.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Hello, I'm a bot",
	})
	return err
}
