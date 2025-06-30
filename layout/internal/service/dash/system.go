package dash

import (
	"context"

	dashv1 "github.com/TBXark/sphere/layout/api/dash/v1"
)

var _ dashv1.SystemServiceHTTPServer = (*Service)(nil)

func (s *Service) ResetCache(ctx context.Context, request *dashv1.ResetCacheRequest) (*dashv1.ResetCacheResponse, error) {
	err := s.cache.DelAll(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.ResetCacheResponse{}, nil
}
