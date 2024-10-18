package config

import (
	"github.com/tbxark/sphere/internal/biz/bot"
	"github.com/tbxark/sphere/internal/pkg/database/client"
	"github.com/tbxark/sphere/internal/server/api"
	"github.com/tbxark/sphere/internal/server/dash"
	"github.com/tbxark/sphere/internal/server/docs"
	"github.com/tbxark/sphere/pkg/log"
	"github.com/tbxark/sphere/pkg/storage/qiniu"
	"github.com/tbxark/sphere/pkg/utils/config/parser"
	"github.com/tbxark/sphere/pkg/utils/secure"
	"github.com/tbxark/sphere/pkg/wechat"
)

var BuildVersion = "dev"

type Config struct {
	Environments map[string]string    `json:"environments" yaml:"environments"`
	Remote       *parser.RemoteConfig `json:"remote" yaml:"remote"`
	Log          *log.Options         `json:"log" yaml:"log"`
	Database     *client.Config       `json:"database" yaml:"database"`
	Dash         *dash.Config         `json:"dash" yaml:"dash"`
	API          *api.Config          `json:"api" yaml:"api"`
	Docs         *docs.Config         `json:"docs" yaml:"docs"`
	Storage      *qiniu.Config        `json:"storage" yaml:"storage"`
	Bot          *bot.Config          `json:"bot" yaml:"bot"`
	WxMini       *wechat.Config       `json:"wx_mini" yaml:"wx_mini"`
}

func NewEmptyConfig() *Config {
	return &Config{
		Environments: map[string]string{
			"GIN_MODE":          "release",
			"CONSUL_HTTP_TOKEN": "",
		},
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
			JWT: secure.RandString(32),
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
		Docs: &docs.Config{
			Address: "0.0.0.0:9999",
			Targets: docs.Targets{
				API:  "http://localhost:8899",
				Dash: "http://localhost:8800",
			},
		},
		Storage: &qiniu.Config{
			AccessKey: "",
			SecretKey: "",
			Bucket:    "",
			Domain:    "",
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
	config, err := parser.Local[Config](path)
	if err != nil {
		return nil, err
	}
	if config.Remote == nil {
		return config, nil
	}
	config, err = parser.Remote[Config](config.Remote)
	if err != nil {
		return nil, err
	}
	return setDefaultConfig(config), nil
}
