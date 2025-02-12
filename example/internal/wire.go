package internal

import (
	"github.com/TBXark/sphere/cache"
	"github.com/TBXark/sphere/cache/memory"
	"github.com/TBXark/sphere/example/internal/biz"
	"github.com/TBXark/sphere/example/internal/config"
	"github.com/TBXark/sphere/example/internal/pkg"
	"github.com/TBXark/sphere/example/internal/server"
	"github.com/TBXark/sphere/example/internal/service"
	"github.com/TBXark/sphere/storage"
	"github.com/TBXark/sphere/storage/qiniu"
	"github.com/TBXark/sphere/wechat"
	"github.com/google/wire"
)

var cacheSet = wire.NewSet(
	memory.NewByteCache,
	wire.Bind(new(cache.ByteCache), new(*memory.Cache[[]byte])),
)

var storageSet = wire.NewSet(
	qiniu.NewQiniu,
	wire.Bind(new(storage.Storage), new(*qiniu.Qiniu)),
)

var ProviderSet = wire.NewSet(
	wire.NewSet(wechat.NewWechat, storageSet, cacheSet),
	server.ProviderSet, service.ProviderSet, pkg.ProviderSet, biz.ProviderSet, config.ProviderSet,
)
