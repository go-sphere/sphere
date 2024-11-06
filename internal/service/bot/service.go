package bot

import (
	"context"
	botv1 "github.com/tbxark/sphere/api/bot/v1"
)

var _ botv1.BotServiceServer = &Service{}

type Service struct {
}

func NewService() *Service {
	return &Service{}
}

func (s Service) Counter(ctx context.Context, request *botv1.CounterRequest) (*botv1.CounterResponse, error) {
	return &botv1.CounterResponse{
		Count: request.Count + request.Step,
	}, nil
}

func (s Service) Start(ctx context.Context, request *botv1.StartRequest) (*botv1.StartResponse, error) {
	return &botv1.StartResponse{
		Message: "Hello " + request.Name,
	}, nil
}
