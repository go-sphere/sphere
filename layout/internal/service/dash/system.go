package dash

import (
	"context"
	dashv2 "github.com/TBXark/sphere/layout/api/dash/v1"
)

var _ dashv2.SystemServiceHTTPServer = (*Service)(nil)

func (s *Service) CacheReset(ctx context.Context, req *dashv2.CacheResetRequest) (*dashv2.CacheResetResponse, error) {
	err := s.Cache.DelAll(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv2.CacheResetResponse{}, nil
}

func (s *Service) MenuAll(ctx context.Context, request *dashv2.MenuAllRequest) (*dashv2.MenuAllResponse, error) {
	return &dashv2.MenuAllResponse{}, nil
}
