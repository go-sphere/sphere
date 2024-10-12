//go:build wireinject
// +build wireinject

package pkg

import (
	"github.com/google/wire"
	"github.com/tbxark/go-base-api/pkg/cache"
	"github.com/tbxark/go-base-api/pkg/cache/memory"
	"github.com/tbxark/go-base-api/pkg/dao/client"
	"github.com/tbxark/go-base-api/pkg/storage"
	"github.com/tbxark/go-base-api/pkg/storage/qiniu"
	"github.com/tbxark/go-base-api/pkg/wechat"
)

var cacheSet = wire.NewSet(
	memory.NewByteCache,
	wire.Bind(new(cache.ByteCache), new(*memory.Cache[[]byte])),
)

var storageSet = wire.NewSet(
	qiniu.NewQiniu,
	wire.Bind(new(storage.Storage), new(*qiniu.Qiniu)),
)

var ProviderSet = wire.NewSet(client.NewDbClient, wechat.NewWechat, storageSet, cacheSet)
