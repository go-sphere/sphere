package bot

import (
	"context"
	botv2 "github.com/TBXark/sphere/example/api/bot/v1"
)

var _ botv2.CounterServiceBotServer = &Service{}

type Service struct {
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Counter(ctx context.Context, request *botv2.CounterRequest) (*botv2.CounterResponse, error) {
	return &botv2.CounterResponse{
		Count: request.Count + request.Step,
	}, nil
}

func (s *Service) Start(ctx context.Context, request *botv2.StartRequest) (*botv2.StartResponse, error) {
	return &botv2.StartResponse{
		Message: "Hello " + request.Name,
	}, nil
}
