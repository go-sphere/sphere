package wechat

import (
	"context"
	"time"

	"github.com/go-sphere/sphere/cache/mcache"
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

func (w *Wechat) GetJsTicket(ctx context.Context, reload bool) (string, error) {
	key := "JsTicket"
	if !reload {
		token, exist, err := w.cache.Get(ctx, key)
		if err != nil {
			return "", err
		}
		if exist {
			return token, nil
		}
	}
	ticket, err, _ := w.sf.Do(key, func() (interface{}, error) {
		ticket, err := withAccessToken[JsTicketResponse](ctx, w, func(ctx context.Context, accessToken string) (*JsTicketResponse, error) {
			resp, err := w.client.R().
				Clone(ctx).
				SetQueryParams(map[string]string{
					"access_token": accessToken,
					"type":         "jsapi",
				}).
				Get("/cgi-bin/ticket/getticket")
			if err != nil {
				return nil, err
			}
			return loadSuccessResponse(resp, func(a *JsTicketResponse) error {
				return checkResponseError(a.ErrCode, a.ErrMsg)
			})
		})
		if err != nil {
			return nil, err
		}
		_ = w.cache.SetWithTTL(ctx, key, ticket.Ticket, time.Duration(ticket.ExpiresIn-2)*time.Second) // 提前2秒过期，避免在过期时请求失败
		return ticket, nil
	})
	if err != nil {
		return "", err
	}
	return ticket.(*JsTicketResponse).Ticket, nil
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
			opts.retryable = false
			opts.reloadAccessToken = true
			return withAccessToken[T](ctx, w, task, WithClone(opts))
		}
		return nil, err
	}
	return resp, nil
}
