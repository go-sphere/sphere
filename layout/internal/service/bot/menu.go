package bot

import (
	"context"

	botv1 "github.com/TBXark/sphere/layout/api/bot/v1"
)

var _ botv1.MenuServiceBotServer = (*Service)(nil)

func (s Service) UpdateCount(ctx context.Context, request *botv1.UpdateCountRequest) (*botv1.UpdateCountResponse, error) {
	return &botv1.UpdateCountResponse{
		Value: request.Value + request.Offset,
	}, nil
}
