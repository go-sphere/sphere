package bot

import (
	"context"
	"fmt"
	botv2 "github.com/TBXark/sphere/example/api/bot/v1"
	"github.com/TBXark/sphere/telegram"
	"math/rand"
)

var _ botv2.CounterServiceBotCodec = &CounterServiceCodec{}

type CounterServiceCodec struct{}

func (b *CounterServiceCodec) DecodeCounterRequest(ctx context.Context, update *telegram.Update) (*botv2.CounterRequest, error) {
	value := UnmarshalUpdateDataWithDefault(update, 0)
	return &botv2.CounterRequest{
		Count: int32(value),
	}, nil
}

func (b *CounterServiceCodec) EncodeCounterResponse(ctx context.Context, reply *botv2.CounterResponse) (*telegram.Message, error) {
	act := botv2.ExtraBotDataCounterServiceCounter.CallbackQuery
	return &telegram.Message{
		Text: fmt.Sprintf("Counter: %d", reply.Count),
		Button: [][]telegram.Button{
			{
				NewButton("Increment", act, reply.Count+1),
				NewButton("Decrement", act, reply.Count-1),
			},
			{
				NewButton("Reset", act, 0),
				NewButton("Random", act, rand.Int()%100),
			},
		},
	}, nil
}

func (b *CounterServiceCodec) DecodeStartRequest(ctx context.Context, update *telegram.Update) (*botv2.StartRequest, error) {
	return &botv2.StartRequest{
		Name: update.Message.From.FirstName,
	}, nil
}

func (b *CounterServiceCodec) EncodeStartResponse(ctx context.Context, reply *botv2.StartResponse) (*telegram.Message, error) {
	return &telegram.Message{
		Text: reply.Message,
	}, nil
}
