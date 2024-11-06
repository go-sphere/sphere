package bot

import (
	"context"
	"fmt"
	"github.com/go-telegram/bot/models"
	botv1 "github.com/tbxark/sphere/api/bot/v1"
	"github.com/tbxark/sphere/pkg/telegram"
	"math/rand"
)

var _ botv1.BotServiceCodec[models.Update, telegram.Message] = &Bot{}

const (
	CommandStart   = "/start"
	CommandCounter = "/counter"
)

const (
	QueryCounter = "counter"
)

func (b *Bot) DecodeCounterRequest(ctx context.Context, update *models.Update) (*botv1.CounterRequest, error) {
	value := UnmarshalUpdateDataWithDefault(update, 0)
	return &botv1.CounterRequest{
		Count: int32(value),
	}, nil
}

func (b *Bot) EncodeCounterResponse(ctx context.Context, reply *botv1.CounterResponse) (*telegram.Message, error) {
	return &telegram.Message{
		Text: fmt.Sprintf("Counter: %d", reply.Count),
		Button: [][]telegram.Button{
			{
				telegram.Button{Text: "+", Type: QueryCounter, Data: reply.Count + 1},
				telegram.Button{Text: "-", Type: QueryCounter, Data: reply.Count - 1},
			},
			{
				telegram.Button{Text: "Reset", Type: QueryCounter, Data: 0},
				telegram.Button{Text: "Random", Type: QueryCounter, Data: rand.Int() % 100},
			},
		},
	}, nil
}

func (b *Bot) DecodeStartRequest(ctx context.Context, update *models.Update) (*botv1.StartRequest, error) {
	return &botv1.StartRequest{
		Name: update.Message.From.FirstName,
	}, nil
}

func (b *Bot) EncodeStartResponse(ctx context.Context, reply *botv1.StartResponse) (*telegram.Message, error) {
	return &telegram.Message{
		Text: fmt.Sprintf("Hello %s", reply.Message),
	}, nil
}
