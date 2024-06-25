package render

import (
	"github.com/tbxark/go-base-api/pkg/dao"
	"github.com/tbxark/go-base-api/pkg/qniu"
)

type Render struct {
	cdn         *qniu.CDN
	db          *dao.Database
	hidePrivacy bool
}

const (
	ImageWidthForAvatar   = 400
	ImageWidthForPlatform = 512
	ImageWidthForCommon   = 1024
)

func NewRender(cdn *qniu.CDN, db *dao.Database, hidePrivacy bool) *Render {
	return &Render{cdn: cdn, db: db, hidePrivacy: hidePrivacy}
}
