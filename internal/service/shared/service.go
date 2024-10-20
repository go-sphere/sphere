package shared

import (
	"github.com/tbxark/sphere/pkg/server/middleware/auth"
	"github.com/tbxark/sphere/pkg/storage"
)

type Service struct {
	Auth       *auth.Auth[int64, string]
	Storage    storage.Storage
	StorageDir string
}

func NewService(auth *auth.Auth[int64, string], store storage.Storage, storageDir string) *Service {
	return &Service{
		Auth:       auth,
		Storage:    store,
		StorageDir: storageDir,
	}
}
