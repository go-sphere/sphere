package wechat

import (
	"context"
	"time"

	"github.com/TBXark/sphere/cache/mcache"
	"golang.org/x/sync/singleflight"
	"resty.dev/v3"
)

type MiniAppEnv string

const (
	MiniAppEnvRelease MiniAppEnv = "release" // 正式版
	MiniAppEnvTrial   MiniAppEnv = "trial"   // 体验版
	MiniAppEnvDevelop MiniAppEnv = "develop" // 开发版
)

func (e MiniAppEnv) String() string {
	return string(e)
}

type Config struct {
	AppID     string     `json:"app_id" yaml:"app_id"`
	AppSecret string     `json:"app_secret" yaml:"app_secret"`
	Proxy     string     `json:"proxy" yaml:"proxy"`
	Env       MiniAppEnv `json:"env" yaml:"env"`
}

type Wechat struct {
	config *Config
	sf     singleflight.Group
	cache  *mcache.Map[string, string]
	client *resty.Client
}

func NewWechat(config *Config) *Wechat {
	if config.Env == "" {
		config.Env = MiniAppEnvRelease
	}
	client := resty.New().
		SetTimeout(time.Second * 30).
		SetBaseURL("https://api.weixin.qq.com")
	if config.Proxy != "" {
		client = client.SetProxy(config.Proxy)
	}
	return &Wechat{
		config: config,
		cache:  mcache.NewMapCache[string](),
		client: client,
	}
}

func (w *Wechat) GetAccessToken(ctx context.Context, reload bool) (string, error) {
	key := "AccessToken"
	if !reload {
		token, exist, err := w.cache.Get(ctx, key)
		if err != nil {
			return "", err
		}
		if exist {
			return token, nil
		}
	}
	token, err, _ := w.sf.Do(key, func() (interface{}, error) {
		resp, err := w.client.R().
			Clone(ctx).
			SetQueryParams(map[string]string{
				"grant_type": "client_credential",
				"appid":      w.config.AppID,
				"secret":     w.config.AppSecret,
			}).
			Get("/cgi-bin/token")
		if err != nil {
			return "", err
		}
		result, err := loadSuccessResponse(resp, func(a *AccessTokenResponse) error {
			return checkResponseError(a.ErrCode, a.ErrMsg)
		})
		if err != nil {
			return "", err
		}
		_ = w.cache.SetWithTTL(ctx, "AccessToken", result.AccessToken, time.Duration(result.ExpiresIn-2)*time.Second) // 提前2秒过期，避免在过期时请求失败
		return result.AccessToken, nil
	})
	if err != nil {
		return "", err
	}
	return token.(string), nil
}

func withAccessToken[T any](ctx context.Context, w *Wechat, task func(ctx context.Context, accessToken string) (*T, error), options ...RequestOption) (*T, error) {
	opts := newRequestOptions(options...)
	token, err := w.GetAccessToken(ctx, opts.reloadAccessToken)
	if err != nil {
		return nil, err
	}
	resp, err := task(ctx, token)
	if err != nil {
		if opts.retryable && isNeedRetryError(err) {
			return withAccessToken[T](ctx, w, task, WithClone(opts), WithRetryable(false))
		}
		return nil, err
	}
	return resp, nil
}

type RequestOptions struct {
	retryable         bool
	reloadAccessToken bool
}

func newRequestOptions(options ...RequestOption) *RequestOptions {
	opts := &RequestOptions{
		retryable:         false,
		reloadAccessToken: false,
	}
	for _, opt := range options {
		opt(opts)
	}
	return opts
}

type RequestOption = func(*RequestOptions)

func WithRetryable(retryable bool) RequestOption {
	return func(opts *RequestOptions) {
		opts.retryable = retryable
	}
}

func WithReloadAccessToken(reload bool) RequestOption {
	return func(opts *RequestOptions) {
		opts.reloadAccessToken = reload
	}
}

func WithClone(opts *RequestOptions) RequestOption {
	return func(o *RequestOptions) {
		o.retryable = opts.retryable
		o.reloadAccessToken = opts.reloadAccessToken
	}
}
