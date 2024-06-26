package client

import (
	"context"
	_ "github.com/go-sql-driver/mysql"
	"github.com/tbxark/go-base-api/pkg/dao/ent"
)

type Config struct {
	Type  string `json:"type"`
	Path  string `json:"path"`
	Debug bool   `json:"debug"`
}

func NewDbClient(config *Config) (*ent.Client, error) {
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
