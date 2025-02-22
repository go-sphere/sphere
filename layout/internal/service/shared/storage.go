package shared

import (
	"context"
	"fmt"
	sharedv1 "github.com/TBXark/sphere/layout/api/shared/v1"
	"github.com/TBXark/sphere/storage"
	"strconv"
)

var _ sharedv1.StorageServiceHTTPServer = (*Service)(nil)

func (s *Service) UploadToken(ctx context.Context, req *sharedv1.UploadTokenRequest) (*sharedv1.UploadTokenResponse, error) {
	if req.Filename == "" {
		return nil, fmt.Errorf("filename is required")
	}
	id, err := s.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	token, err := s.Storage.GenerateUploadToken(req.Filename, s.StorageDir, storage.DefaultKeyBuilder(strconv.Itoa(int(id))))
	if err != nil {
		return nil, err
	}
	return &sharedv1.UploadTokenResponse{
		Token: token[0],
		Key:   token[1],
		Url:   token[2],
	}, nil
}
