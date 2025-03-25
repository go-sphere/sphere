package shared

import (
	"github.com/TBXark/sphere/server/auth/authorizer"
	"github.com/TBXark/sphere/storage"
)

type Service struct {
	authorizer.ContextUtils[int64]
	storage    storage.ImageStorage
	storageDir string
}

func NewService(store storage.ImageStorage, storageDir string) *Service {
	return &Service{
		storage:    store,
		storageDir: storageDir,
	}
}
