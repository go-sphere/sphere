package pkg

import (
	"github.com/google/wire"
	"github.com/tbxark/go-base-api/pkg/cache"
	"github.com/tbxark/go-base-api/pkg/cache/memory"
	"github.com/tbxark/go-base-api/pkg/cdn"
	"github.com/tbxark/go-base-api/pkg/cdn/qiniu"
	"github.com/tbxark/go-base-api/pkg/dao/client"
	"github.com/tbxark/go-base-api/pkg/wechat"
)

var cacheSet = wire.NewSet(
	memory.NewMemoryCache,
	wire.Bind(new(cache.ByteCache), new(*memory.Cache)),
)

var cdnSet = wire.NewSet(
	qiniu.NewQiniu,
	wire.Bind(new(cdn.CDN), new(*qiniu.Qiniu)),
)

var ProviderSet = wire.NewSet(client.NewDbClient, wechat.NewWechat, cdnSet, cacheSet)
