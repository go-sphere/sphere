package bot

import (
	"context"

	botv1 "github.com/TBXark/sphere/layout/api/bot/v1"
)

var _ botv1.MenuServiceBotServer = (*Service)(nil)

func (s Service) Counter(ctx context.Context, request *botv1.CounterRequest) (*botv1.CounterResponse, error) {
	return &botv1.CounterResponse{
		Value: request.Value + request.Offset,
	}, nil
}
