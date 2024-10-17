package config

import (
	"github.com/tbxark/sphere/internal/biz/bot"
	"github.com/tbxark/sphere/internal/pkg/database/client"
	"github.com/tbxark/sphere/internal/server/api"
	"github.com/tbxark/sphere/internal/server/dash"
	"github.com/tbxark/sphere/pkg/log"
	"github.com/tbxark/sphere/pkg/storage/qiniu"
	"github.com/tbxark/sphere/pkg/wechat"
	"math/rand"
	"net/http"
	"os"
	"time"
)

var BuildVersion = "dev"

type RemoteConfig struct {
	URL     string            `json:"url" yaml:"url"`
	Method  string            `json:"method" yaml:"method"`
	Headers map[string]string `json:"headers" yaml:"headers"`
}

type Config struct {
	Environments map[string]string `json:"environments" yaml:"environments"`
	Remote       *RemoteConfig     `json:"remote" yaml:"remote"`
	Log          *log.Options      `json:"log" yaml:"log"`
	Database     *client.Config    `json:"database" yaml:"database"`
	Dash         *dash.Config      `json:"dash" yaml:"dash"`
	API          *api.Config       `json:"api" yaml:"api"`
	Storage      *qiniu.Config     `json:"storage" yaml:"storage"`
	Bot          *bot.Config       `json:"bot" yaml:"bot"`
	WxMini       *wechat.Config    `json:"wx_mini" yaml:"wx_mini"`
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

func LoadLocalConfig(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	config := &Config{}
	err = Unmarshal(Ext(path), file, config)
	if err != nil {
		return nil, err
	}
	return setDefaultConfig(config), nil
}

func LoadRemoteConfig(remote *RemoteConfig) (*Config, error) {
	httpClient := http.Client{
		Timeout: time.Second * 10,
	}
	req, err := http.NewRequest(remote.Method, remote.URL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range remote.Headers {
		req.Header.Add(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	config := &Config{}
	decoder := NewDecoder(Ext(remote.URL), resp.Body)
	if decoder == nil {
		return nil, ErrUnknownCoderType
	}
	err = decoder.Decode(config)
	if err != nil {
		return nil, err
	}
	return setDefaultConfig(config), nil
}
