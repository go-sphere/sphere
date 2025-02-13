package render

import (
	"github.com/TBXark/sphere/layout/internal/pkg/dao"
	"github.com/TBXark/sphere/storage"
)

type Render struct {
	cdn         storage.URLHandler
	db          *dao.Dao
	hidePrivacy bool
}

const (
	ImageWidthForAvatar   = 400
	ImageWidthForPlatform = 512
	ImageWidthForCommon   = 1024
)

func NewRender(cdn storage.URLHandler, db *dao.Dao, hidePrivacy bool) *Render {
	return &Render{cdn: cdn, db: db, hidePrivacy: hidePrivacy}
}
