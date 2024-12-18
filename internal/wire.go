package internal

import (
	"github.com/TBXark/sphere/internal/biz"
	"github.com/TBXark/sphere/internal/config"
	"github.com/TBXark/sphere/internal/pkg"
	"github.com/TBXark/sphere/internal/server"
	"github.com/TBXark/sphere/internal/service"
	"github.com/TBXark/sphere/pkg/cache"
	"github.com/TBXark/sphere/pkg/cache/memory"
	"github.com/TBXark/sphere/pkg/storage"
	"github.com/TBXark/sphere/pkg/storage/qiniu"
	"github.com/TBXark/sphere/pkg/wechat"
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
