package api

import (
	"context"

	apiv1 "github.com/TBXark/sphere/layout/api/api/v1"
)

var _ apiv1.SystemServiceHTTPServer = (*Service)(nil)

func (s *Service) GetStatus(ctx context.Context, request *apiv1.GetStatusRequest) (*apiv1.GetStatusResponse, error) {
	return &apiv1.GetStatusResponse{}, nil
}
