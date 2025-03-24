package client

import (
	"context"

	_ "github.com/TBXark/sphere/database/sqlite"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent"
	_ "github.com/go-sql-driver/mysql"
)

type Config struct {
	Type  string `json:"type" yaml:"type"`
	Path  string `json:"path" yaml:"path"`
	Debug bool   `json:"debug" yaml:"debug"`
}

func NewDataBaseClient(config *Config) (*ent.Client, error) {
	client, err := ent.Open(config.Type, config.Path)
	if err != nil {
		return nil, err
	}
	if e := client.Schema.Create(context.Background()); e != nil {
		return nil, e
	}
	if config.Debug {
		client = client.Debug()
	}
	return client, nil
}
