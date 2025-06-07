package internal

import (
	"github.com/TBXark/sphere/cache"
	"github.com/TBXark/sphere/cache/memory"
	"github.com/TBXark/sphere/layout/internal/biz"
	"github.com/TBXark/sphere/layout/internal/config"
	"github.com/TBXark/sphere/layout/internal/pkg"
	"github.com/TBXark/sphere/layout/internal/server"
	"github.com/TBXark/sphere/layout/internal/service"
	"github.com/TBXark/sphere/server/service/file"
	"github.com/TBXark/sphere/social/wechat"
	"github.com/TBXark/sphere/storage"
	"github.com/TBXark/sphere/storage/fileserver"
	"github.com/google/wire"
)

var cacheSet = wire.NewSet(
	memory.NewByteCache,
	wire.Bind(new(cache.ByteCache), new(*memory.ByteCache)),
)

var storageSet = wire.NewSet(
	file.NewWebServer,        // If you want to use the local file to s3 adapter server, you can use this line
	file.NewLocalFileService, // Wrapper for local file storage to S3 adapter
	wire.Bind(new(storage.CDNStorage), new(*fileserver.S3Adapter)), // Bind the S3Adapter to the CDNStorage interface
)

var ProviderSet = wire.NewSet(
	// Sphere library components
	wire.NewSet(
		storageSet,
		cacheSet,
		wechat.NewWechat,
	),
	// Internal application components
	wire.NewSet(
		server.ProviderSet,
		service.ProviderSet,
		pkg.ProviderSet,
		biz.ProviderSet,
		config.ProviderSet,
	),
)
