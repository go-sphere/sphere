package pkg

import (
	"github.com/google/wire"
	"github.com/tbxark/go-base-api/pkg/cache"
	"github.com/tbxark/go-base-api/pkg/dao/client"
	"github.com/tbxark/go-base-api/pkg/qniu"
	"github.com/tbxark/go-base-api/pkg/wechat"
)

var ProviderSet = wire.NewSet(client.NewDbClient, qniu.NewCDN, wechat.NewWechat, cache.NewCache)
