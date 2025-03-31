package render

import (
	"github.com/TBXark/sphere/layout/internal/pkg/dao"
	"github.com/TBXark/sphere/storage"
)

type Render struct {
	db          *dao.Dao
	storage     storage.ImageURLHandler
	hidePrivacy bool
}

const (
	ImageWidthForAvatar   = 400
	ImageWidthForPlatform = 512
	ImageWidthForCommon   = 1024
)

func NewRender(db *dao.Dao, storage storage.ImageURLHandler, hidePrivacy bool) *Render {
	return &Render{db: db, storage: storage, hidePrivacy: hidePrivacy}
}
