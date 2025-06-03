package internal

import (
	"github.com/TBXark/sphere/cache"
	"github.com/TBXark/sphere/cache/memory"
	"github.com/TBXark/sphere/layout/internal/biz"
	"github.com/TBXark/sphere/layout/internal/config"
	"github.com/TBXark/sphere/layout/internal/pkg"
	"github.com/TBXark/sphere/layout/internal/pkg/file"
	"github.com/TBXark/sphere/layout/internal/server"
	"github.com/TBXark/sphere/layout/internal/service"
	"github.com/TBXark/sphere/storage"
	"github.com/TBXark/sphere/wechat"
	"github.com/google/wire"
)

var cacheSet = wire.NewSet(
	memory.NewByteCache,
	wire.Bind(new(cache.ByteCache), new(*memory.ByteCache)),
)

var storageSet = wire.NewSet(
	file.NewFileService,
	wire.Bind(new(storage.CDNStorage), new(*file.Service)),
)

var ProviderSet = wire.NewSet(
	wire.NewSet(
		wechat.NewWechat,
		storageSet,
		cacheSet,
	),
	server.ProviderSet,
	service.ProviderSet,
	pkg.ProviderSet,
	biz.ProviderSet,
	config.ProviderSet,
)
