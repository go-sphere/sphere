package api

import (
	"context"
	apiv2 "github.com/TBXark/sphere/example/api/api/v1"
)

var _ apiv2.SystemServiceHTTPServer = (*Service)(nil)

func (s *Service) Status(ctx context.Context, req *apiv2.StatusRequest) (*apiv2.StatusResponse, error) {
	return &apiv2.StatusResponse{}, nil
}
