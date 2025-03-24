package bot

import (
	"context"

	botv1 "github.com/TBXark/sphere/layout/api/bot/v1"
)

var _ botv1.MenuServiceBotServer = &Service{}

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Start(ctx context.Context, request *botv1.StartRequest) (*botv1.StartResponse, error) {
	return &botv1.StartResponse{
		Message: "Hello " + request.Name,
	}, nil
}
