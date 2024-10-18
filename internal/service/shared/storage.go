package shared

import (
	"context"
	"fmt"
	sharedv1 "github.com/tbxark/sphere/api/shared/v1"
	"github.com/tbxark/sphere/pkg/storage"
	"strconv"
)

var _ sharedv1.StorageServiceHTTPServer = (*Service)(nil)

func (s *Service) UploadToken(ctx context.Context, req *sharedv1.UploadTokenRequest) (*sharedv1.UploadTokenResponse, error) {
	if req.Filename == "" {
		return nil, fmt.Errorf("filename is required")
	}
	id, err := s.Auth.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	token := s.Storage.GenerateUploadToken(req.Filename, s.StorageDir, storage.DefaultKeyBuilder(strconv.Itoa(int(id))))
	return &sharedv1.UploadTokenResponse{
		Token: token.Token,
		Key:   token.Key,
		Url:   token.URL,
	}, nil
}
