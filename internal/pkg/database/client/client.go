package client

import (
	"context"
	_ "github.com/go-sql-driver/mysql"
	"github.com/tbxark/sphere/internal/pkg/database/ent"
	_ "github.com/tbxark/sphere/pkg/database/sqlite"
)

type Config struct {
	Type  string `json:"type"`
	Path  string `json:"path"`
	Debug bool   `json:"debug"`
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
