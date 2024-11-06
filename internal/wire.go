package internal

import (
	"github.com/google/wire"
	"github.com/tbxark/sphere/internal/biz"
	"github.com/tbxark/sphere/internal/config"
	"github.com/tbxark/sphere/internal/pkg"
	"github.com/tbxark/sphere/internal/server"
	"github.com/tbxark/sphere/internal/service"
	"github.com/tbxark/sphere/pkg/cache"
	"github.com/tbxark/sphere/pkg/cache/memory"
	"github.com/tbxark/sphere/pkg/storage"
	"github.com/tbxark/sphere/pkg/storage/qiniu"
	"github.com/tbxark/sphere/pkg/wechat"
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
