package shared

import (
	"github.com/tbxark/sphere/pkg/storage"
	"github.com/tbxark/sphere/pkg/web/middleware/auth"
)

type Service struct {
	Auth       *auth.Auth
	Storage    storage.Storage
	StorageDir string
}

func NewService(auth *auth.Auth, store storage.Storage, storageDir string) *Service {
	return &Service{
		Auth:       auth,
		Storage:    store,
		StorageDir: storageDir,
	}
}
