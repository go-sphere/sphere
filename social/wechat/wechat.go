package wechat

import (
	"context"
	"time"

	"github.com/go-sphere/sphere/cache/mcache"
	"golang.org/x/sync/singleflight"
	"resty.dev/v3"
)

// MiniAppEnv represents the environment type for WeChat Mini Programs.
type MiniAppEnv string

const (
	// MiniAppEnvRelease represents the production environment for Mini Programs.
	MiniAppEnvRelease MiniAppEnv = "release" // 正式版
	// MiniAppEnvTrial represents the trial/staging environment for Mini Programs.
	MiniAppEnvTrial MiniAppEnv = "trial" // 体验版
	// MiniAppEnvDevelop represents the development environment for Mini Programs.
	MiniAppEnvDevelop MiniAppEnv = "develop" // 开发版
)

// String returns the string representation of the MiniAppEnv.
func (e MiniAppEnv) String() string {
	return string(e)
}

// Config holds the configuration parameters for WeChat API integration.
type Config struct {
	AppID     string     `json:"app_id" yaml:"app_id"`         // WeChat application ID
	AppSecret string     `json:"app_secret" yaml:"app_secret"` // WeChat application secret
	Proxy     string     `json:"proxy" yaml:"proxy"`           // Optional proxy server URL
	Env       MiniAppEnv `json:"env" yaml:"env"`               // Mini Program environment
}

// Wechat represents a WeChat API client with token management and caching capabilities.
// It handles access token lifecycle, API requests, and provides thread-safe operations.
type Wechat struct {
	config *Config                     // WeChat application configuration
	sf     singleflight.Group          // Prevents duplicate token requests
	cache  *mcache.Map[string, string] // Cache for access tokens and tickets
	client *resty.Client               // HTTP client for WeChat API requests
}

// NewWechat creates a new WeChat API client with the provided configuration.
// It initializes the HTTP client with appropriate timeouts, base URL, and optional proxy settings.
// If no environment is specified, it defaults to the release environment.
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

// GetAccessToken retrieves a valid WeChat access token, using cache when possible.
// It implements automatic token refresh with singleflight to prevent duplicate requests.
// The token is cached with a 2-second safety margin before the actual expiration time.
//
// Parameters:
//   - ctx: Context for request cancellation and timeout
//   - reload: Forces token refresh if true, bypassing cache
//
// Returns the access token string or an error if retrieval fails.
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

// GetJsTicket retrieves a valid JS-SDK ticket for WeChat web applications.
// Similar to GetAccessToken, it uses caching and singleflight for efficiency.
// The ticket is required for WeChat JS-SDK initialization in web pages.
//
// Parameters:
//   - ctx: Context for request cancellation and timeout
//   - reload: Forces ticket refresh if true, bypassing cache
//
// Returns the JS ticket string or an error if retrieval fails.
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
