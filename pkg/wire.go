package pkg

import (
	"github.com/google/wire"
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

var ProviderSet = wire.NewSet(wechat.NewWechat, storageSet, cacheSet)
