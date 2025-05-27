package shared

import (
	"github.com/TBXark/sphere/server/auth/authorizer"
	"github.com/TBXark/sphere/storage"
)

type Service struct {
	authorizer.ContextUtils[int64]
	storage    storage.CDNStorage
	storageDir string
}

func NewService(storage storage.CDNStorage, storageDir string) *Service {
	return &Service{
		storage:    storage,
		storageDir: storageDir,
	}
}
