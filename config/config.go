package config

import (
	"encoding/json"
	"github.com/spf13/viper"
	"github.com/tbxark/go-base-api/internal/biz/api"
	"github.com/tbxark/go-base-api/internal/biz/bot"
	"github.com/tbxark/go-base-api/internal/biz/dash"
	"github.com/tbxark/go-base-api/pkg/cdn/qiniu"
	"github.com/tbxark/go-base-api/pkg/dao/client"
	"github.com/tbxark/go-base-api/pkg/log"
	"github.com/tbxark/go-base-api/pkg/wechat"
	"math/rand"
	"os"
)

var BuildVersion = "dev"

type SystemConfig struct {
	GinMode string `json:"gin_mode"`
}

type RemoteConfig struct {
	Provider string `json:"provider"`
	Endpoint string `json:"endpoint"`
	Path     string `json:"path"`
}

type Config struct {
	System   *SystemConfig  `json:"system"`
	Remote   *RemoteConfig  `json:"remote"`
	Log      *log.Options   `json:"log"`
	Database *client.Config `json:"database"`
	Dash     *dash.Config   `json:"dash"`
	API      *api.Config    `json:"api"`
	CDN      *qiniu.Config  `json:"cdn"`
	Bot      *bot.Config    `json:"bot"`
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
		Database: &client.Config{},
		Dash: &dash.Config{
			Address: "0.0.0.0:8800",
			JWT:     randJWTPassword(),
		},
		API: &api.Config{
			Address: "0.0.0.0:8899",
			JWT:     randJWTPassword(),
		},
		CDN: &qiniu.Config{
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

func setDefaultConfig(config *Config) *Config {
	if config.System == nil {
		config.System = &SystemConfig{
			GinMode: "release",
		}
	}
	if config.Log == nil {
		config.Log = log.NewOptions()
	}
	return config
}

func LoadLocalConfig(path string) (*Config, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	config := &Config{}
	err = json.Unmarshal(bytes, config)
	if err != nil {
		return nil, err
	}
	return setDefaultConfig(config), nil
}

func LoadRemoteConfig(provider, endpoint, path string) (*Config, error) {
	err := viper.AddRemoteProvider(provider, endpoint, path)
	if err != nil {
		return nil, err
	}
	viper.SetConfigType("json")
	err = viper.ReadRemoteConfig()
	if err != nil {
		return nil, err
	}
	config := &Config{}
	err = viper.Unmarshal(config)
	if err != nil {
		return nil, err
	}
	return setDefaultConfig(config), nil
}
