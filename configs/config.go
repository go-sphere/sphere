package configs

import (
	"encoding/json"
	"github.com/spf13/viper"
	_ "github.com/spf13/viper/remote"
	"github.com/tbxark/go-base-api/internal/biz/api"
	"github.com/tbxark/go-base-api/internal/biz/bot"
	"github.com/tbxark/go-base-api/internal/biz/dash"
	"github.com/tbxark/go-base-api/pkg/dao/client"
	"github.com/tbxark/go-base-api/pkg/log"
	"github.com/tbxark/go-base-api/pkg/storage/qiniu"
	"github.com/tbxark/go-base-api/pkg/wechat"
	"math/rand"
	"os"
)

var BuildVersion = "dev"

type RemoteConfig struct {
	Provider   string `json:"provider"`
	Endpoint   string `json:"endpoint"`
	Path       string `json:"path"`
	ConfigType string `json:"config_type"`
	SecretKey  string `json:"secret"`
}

type Config struct {
	Environments map[string]string `json:"environments"`
	Remote       *RemoteConfig     `json:"remote"`
	Log          *log.Options      `json:"log"`
	Database     *client.Config    `json:"database"`
	Dash         *dash.Config      `json:"dash"`
	API          *api.Config       `json:"api"`
	Storage      *qiniu.Config     `json:"storage"`
	Bot          *bot.Config       `json:"bot"`
	WxMini       *wechat.Config    `json:"wx_mini"`
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
		Environments: map[string]string{
			"GIN_MODE":          "release",
			"CONSUL_HTTP_TOKEN": "",
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
		Storage: &qiniu.Config{
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

func LoadRemoteConfig(remote *RemoteConfig) (*Config, error) {
	viper.SetConfigType(remote.ConfigType)
	err := viper.AddSecureRemoteProvider(remote.Provider, remote.Endpoint, remote.Path, remote.SecretKey)
	if err != nil {
		return nil, err
	}
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
