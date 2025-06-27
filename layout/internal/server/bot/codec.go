package bot

import (
	"context"
	"fmt"
	botv1 "github.com/TBXark/sphere/layout/api/bot/v1"
	"github.com/TBXark/sphere/social/telegram"
)

var _ botv1.MenuServiceBotCodec = &MenuServiceBotCodec{}

type MenuServiceBotCodec struct{}

func (m MenuServiceBotCodec) DecodeCounterRequest(ctx context.Context, request *telegram.Update) (*botv1.CounterRequest, error) {
	res := UnmarshalUpdateDataWithDefault[botv1.CounterRequest](request, botv1.CounterRequest{})
	return &res, nil
}

func (m MenuServiceBotCodec) EncodeCounterResponse(ctx context.Context, response *botv1.CounterResponse) (*telegram.Message, error) {
	return &telegram.Message{
		Text:      fmt.Sprintf("Counter: %d", response.Value),
		Media:     nil,
		ParseMode: "",
		Button: [][]telegram.Button{
			{
				NewButtonX("-1", botv1.ExtraBotDataMenuServiceCounter, botv1.CounterRequest{
					Value:  response.Value,
					Offset: -1,
				}),
				NewButtonX("+1", botv1.ExtraBotDataMenuServiceCounter, botv1.CounterRequest{
					Value:  response.Value,
					Offset: 1,
				}),
			},
			{
				NewButtonX("Reset", botv1.ExtraBotDataMenuServiceCounter, botv1.CounterRequest{
					Value:  0,
					Offset: 0,
				}),
			},
		},
	}, nil
}
