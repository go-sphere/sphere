package bot

import (
	"context"
	"fmt"

	botv1 "github.com/TBXark/sphere/layout/api/bot/v1"
	"github.com/TBXark/sphere/social/telegram"
)

var _ botv1.MenuServiceBotCodec = &MenuServiceBotCodec{}

type MenuServiceBotCodec struct{}

func (m MenuServiceBotCodec) DecodeUpdateCountRequest(ctx context.Context, request *telegram.Update) (*botv1.UpdateCountRequest, error) {
	res := UnmarshalUpdateDataWithDefault[botv1.UpdateCountRequest](request, botv1.UpdateCountRequest{})
	return &res, nil
}

func (m MenuServiceBotCodec) EncodeUpdateCountResponse(ctx context.Context, response *botv1.UpdateCountResponse) (*telegram.Message, error) {
	return &telegram.Message{
		Text:      fmt.Sprintf("UpdateCount: %d", response.Value),
		Media:     nil,
		ParseMode: "",
		Button: [][]telegram.Button{
			{
				NewButtonX("-1", botv1.ExtraBotDataMenuServiceUpdateCount, botv1.UpdateCountRequest{
					Value:  response.Value,
					Offset: -1,
				}),
				NewButtonX("+1", botv1.ExtraBotDataMenuServiceUpdateCount, botv1.UpdateCountRequest{
					Value:  response.Value,
					Offset: 1,
				}),
			},
			{
				NewButtonX("Reset", botv1.ExtraBotDataMenuServiceUpdateCount, botv1.UpdateCountRequest{
					Value:  0,
					Offset: 0,
				}),
			},
		},
	}, nil
}
