package config

import (
	"github.com/TBXark/confstore"
	"github.com/TBXark/sphere/layout/internal/pkg/database/client"
	"github.com/TBXark/sphere/layout/internal/server/api"
	"github.com/TBXark/sphere/layout/internal/server/bot"
	"github.com/TBXark/sphere/layout/internal/server/dash"
	"github.com/TBXark/sphere/layout/internal/server/docs"
	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/server/service/file"
	"github.com/TBXark/sphere/social/wechat"
	"github.com/TBXark/sphere/storage/local"
	"github.com/TBXark/sphere/utils/secure"
)

var BuildVersion = "dev"

type Config struct {
	Environments map[string]string `json:"environments" yaml:"environments"`
	Log          *log.Options      `json:"log" yaml:"log"`
	Database     *client.Config    `json:"database" yaml:"database"`
	Dash         *dash.Config      `json:"dash" yaml:"dash"`
	API          *api.Config       `json:"api" yaml:"api"`
	File         *file.Config      `json:"file" yaml:"file"`
	Docs         *docs.Config      `json:"docs" yaml:"docs"`
	Storage      *local.Config     `json:"storage" yaml:"storage"`
	Bot          *bot.Config       `json:"bot" yaml:"bot"`
	WxMini       *wechat.Config    `json:"wx_mini" yaml:"wx_mini"`
}

func NewEmptyConfig() *Config {
	return &Config{
		Environments: map[string]string{},
		Log: &log.Options{
			File: &log.FileOptions{
				FileName:   "./var/log/sphere.log",
				MaxSize:    10,
				MaxBackups: 10,
				MaxAge:     10,
			},
			Level: "info",
		},
		Database: &client.Config{},
		Dash: &dash.Config{
			AuthJWT:    secure.RandString(32),
			RefreshJWT: secure.RandString(32),
			HTTP: dash.HTTPConfig{
				Address: "0.0.0.0:8800",
			},
		},
		API: &api.Config{
			JWT: secure.RandString(32),
			HTTP: api.HTTPConfig{
				Address: "0.0.0.0:8899",
			},
		},
		File: &file.Config{
			HTTP: file.HTTPConfig{
				Address: "0.0.0.0:9900",
				Cors: []string{
					"http://localhost:*",
				},
			},
		},
		Docs: &docs.Config{
			Address: "0.0.0.0:9999",
			Targets: docs.Targets{
				API:  "http://localhost:8899",
				Dash: "http://localhost:8800",
			},
		},
		Storage: &local.Config{
			RootDir:    "./var/file",
			PublicBase: "http://localhost:9900",
		},
		Bot: &bot.Config{
			Token: "",
		},
		WxMini: &wechat.Config{
			AppID:     "",
			AppSecret: "",
			Env:       "develop",
		},
	}
}

func setDefaultConfig(config *Config) *Config {
	if config.Log == nil {
		config.Log = log.NewOptions()
	}
	return config
}

func NewConfig(path string) (*Config, error) {
	config, err := confstore.Load[Config](path)
	if err != nil {
		return nil, err
	}
	return setDefaultConfig(config), nil
}
