package file

import (
	"github.com/TBXark/sphere/cache/memory"
	"github.com/TBXark/sphere/storage/fileserver"
	"github.com/TBXark/sphere/storage/local"
)

type Config = local.Config

type Service = fileserver.S3Adapter

func NewFileService(config *Config) (*Service, error) {
	client, err := local.NewClient(config)
	if err != nil {
		return nil, err
	}
	adapter := fileserver.NewS3Adapter(
		&fileserver.Config{PublicBase: config.PublicBase},
		memory.NewByteCache(),
		client,
	)
	return adapter, nil
}
