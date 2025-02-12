package shared

import (
	"context"
	"fmt"
	sharedv2 "github.com/TBXark/sphere/layout/api/shared/v1"
	"github.com/TBXark/sphere/storage"
	"strconv"
)

var _ sharedv2.StorageServiceHTTPServer = (*Service)(nil)

func (s *Service) UploadToken(ctx context.Context, req *sharedv2.UploadTokenRequest) (*sharedv2.UploadTokenResponse, error) {
	if req.Filename == "" {
		return nil, fmt.Errorf("filename is required")
	}
	id, err := s.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	token := s.Storage.GenerateUploadToken(req.Filename, s.StorageDir, storage.DefaultKeyBuilder(strconv.Itoa(int(id))))
	return &sharedv2.UploadTokenResponse{
		Token:	token.Token,
		Key:	token.Key,
		Url:	token.URL,
	}, nil
}
