package bot

import (
	"context"

	botv1 "github.com/TBXark/sphere/layout/api/bot/v1"
	"github.com/TBXark/sphere/telegram"
)

var _ botv1.MenuServiceBotCodec = &MenuServiceBotCodec{}

type MenuServiceBotCodec struct{}

func (b *MenuServiceBotCodec) DecodeStartRequest(ctx context.Context, update *telegram.Update) (*botv1.StartRequest, error) {
	return &botv1.StartRequest{
		Name: update.Message.From.FirstName,
	}, nil
}

func (b *MenuServiceBotCodec) EncodeStartResponse(ctx context.Context, reply *botv1.StartResponse) (*telegram.Message, error) {
	return &telegram.Message{
		Text: reply.Message,
	}, nil
}
