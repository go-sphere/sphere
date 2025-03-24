package api

import (
	"context"

	apiv1 "github.com/TBXark/sphere/layout/api/api/v1"
)

var _ apiv1.SystemServiceHTTPServer = (*Service)(nil)

func (s *Service) Status(ctx context.Context, req *apiv1.StatusRequest) (*apiv1.StatusResponse, error) {
	return &apiv1.StatusResponse{}, nil
}
