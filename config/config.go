package config

import (
	"encoding/json"
	"math/rand"
	"os"

	"github.com/tbxark/go-base-api/internal/biz/api"
	"github.com/tbxark/go-base-api/internal/biz/dash"
	"github.com/tbxark/go-base-api/pkg/dao"
	"github.com/tbxark/go-base-api/pkg/log"
	"github.com/tbxark/go-base-api/pkg/qniu"
	"github.com/tbxark/go-base-api/pkg/wechat"
)

var BuildVersion = "dev"

type SystemConfig struct {
	GinMode string `json:"gin_mode"`
}

type Config struct {
	System   *SystemConfig  `json:"system"`
	Log      *log.Options   `json:"log"`
	Database *dao.Config    `json:"database"`
	Dash     *dash.Config   `json:"dash"`
	API      *api.Config    `json:"api"`
	CDN      *qniu.Config   `json:"cdn"`
	WxMini   *wechat.Config `json:"wx_mini"`
}

func NewEmptyConfig() *Config {
	randJWTPassword := func() string {
		jwtChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		jwt := make([]byte, 32)
		for i := range jwt {
			jwt[i] = jwtChars[rand.Intn(len(jwtChars))]
		}
		return string(jwt)
	}
	return &Config{
		System: &SystemConfig{
			GinMode: "debug",
		},
		Log: &log.Options{
			File: &log.FileOptions{
				FileName:   "/var/log/go-base-api.log",
				MaxSize:    10,
				MaxBackups: 10,
				MaxAge:     10,
			},
			Level: "info",
		},
		Database: &dao.Config{},
		Dash: &dash.Config{
			Address: "127.0.0.1:8800",
			JWT:     randJWTPassword(),
		},
		API: &api.Config{
			Address: "127.0.0.1:8899",
			JWT:     randJWTPassword(),
		},
		CDN: &qniu.Config{
			AccessKey: "",
			SecretKey: "",
			Bucket:    "",
			Domain:    "",
		},
		WxMini: &wechat.Config{
			AppID:     "",
			AppSecret: "",
			Env:       "develop",
		},
	}
}

func LoadConfig(path string) (*Config, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	config := &Config{}
	err = json.Unmarshal(bytes, config)
	if err != nil {
		return nil, err
	}
	if config.System == nil {
		config.System = &SystemConfig{
			GinMode: "release",
		}
	}
	if config.Log == nil {
		config.Log = log.NewOptions()
	}
	return config, nil
}
