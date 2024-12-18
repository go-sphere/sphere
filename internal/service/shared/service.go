package shared

import (
	"github.com/TBXark/sphere/pkg/server/auth/authorizer"
	"github.com/TBXark/sphere/pkg/storage"
)

type Service struct {
	authorizer.ContextUtils[int64]
	Storage    storage.Storage
	StorageDir string
}

func NewService(store storage.Storage, storageDir string) *Service {
	return &Service{
		Storage:    store,
		StorageDir: storageDir,
	}
}
