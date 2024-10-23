package shared

import (
	"github.com/tbxark/sphere/pkg/storage"
)

type Service struct {
	Storage    storage.Storage
	StorageDir string
}

func NewService(store storage.Storage, storageDir string) *Service {
	return &Service{
		Storage:    store,
		StorageDir: storageDir,
	}
}
