package shared

import (
	"context"
	"fmt"
	"strconv"

	sharedv1 "github.com/TBXark/sphere/layout/api/shared/v1"
	"github.com/TBXark/sphere/storage"
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
	key := storage.DefaultKeyBuilder(strconv.Itoa(int(id)))
	token, err := s.storage.GenerateUploadToken(ctx, req.Filename, s.storageDir, key)
	if err != nil {
		return nil, err
	}
	return &sharedv1.UploadTokenResponse{
		Token: token[0],
		Key:   token[1],
		Url:   token[2],
	}, nil
}
