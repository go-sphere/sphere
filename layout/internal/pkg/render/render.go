package render

import (
	"github.com/TBXark/sphere/layout/internal/pkg/dao"
	"github.com/TBXark/sphere/storage"
)

type Render struct {
	db          *dao.Dao
	storage     storage.URLHandler
	hidePrivacy bool
}

func NewRender(db *dao.Dao, storage storage.URLHandler, hidePrivacy bool) *Render {
	return &Render{db: db, storage: storage, hidePrivacy: hidePrivacy}
}
