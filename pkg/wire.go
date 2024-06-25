package pkg

import (
	"github.com/google/wire"
	"github.com/tbxark/go-base-api/pkg/cache"
	"github.com/tbxark/go-base-api/pkg/dao"
	"github.com/tbxark/go-base-api/pkg/qniu"
	"github.com/tbxark/go-base-api/pkg/wechat"
)

var ProviderSet = wire.NewSet(dao.NewDatabase, qniu.NewCDN, wechat.NewWechat, cache.NewCache)
