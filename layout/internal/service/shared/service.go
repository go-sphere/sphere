package shared

import (
	"github.com/TBXark/sphere/server/auth/authorizer"
	"github.com/TBXark/sphere/storage"
)

type Service struct {
	authorizer.ContextUtils[int64]
	Storage    storage.ImageStorage
	StorageDir string
}

func NewService(store storage.ImageStorage, storageDir string) *Service {
	return &Service{
		Storage:    store,
		StorageDir: storageDir,
	}
}
