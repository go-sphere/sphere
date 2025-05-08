package wechat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

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
	Env       MiniAppEnv `json:"env" yaml:"env"`
}

type Wechat struct {
	config            *Config
	accessToken       string
	accessTokenExpire time.Time
	client            *resty.Client
}

func NewWechat(config *Config) *Wechat {
	if config.Env == "" {
		config.Env = MiniAppEnvRelease
	}
	return &Wechat{
		config: config,
		client: resty.New().
			SetTimeout(time.Second * 30).
			SetBaseURL("https://api.weixin.qq.com"),
	}
}

func loadSuccessResponse[T any](resp *resty.Response, check func(*T) error) (*T, error) {
	if resp.IsError() {
		result := resp.Error().(*EmptyResponse)
		if result.ErrCode != 0 {
			return nil, checkResponseError(result.ErrCode, result.ErrMsg)
		}
		return nil, fmt.Errorf("unknown error: %s", resp.Status())
	}
	if resp.IsSuccess() {
		result := resp.Result().(*T)
		err := check(result)
		if err != nil {
			return nil, err
		}
		return result, nil
	}
	return nil, fmt.Errorf("unknown error: %s", resp.Status())
}

func (w *Wechat) Auth(ctx context.Context, code string) (*AuthResponse, error) {
	resp, err := w.client.R().
		Clone(ctx).
		SetQueryParams(map[string]string{
			"appid":      w.config.AppID,
			"secret":     w.config.AppSecret,
			"js_code":    code,
			"grant_type": "authorization_code",
		}).
		SetResult(&AuthResponse{}).
		SetError(&EmptyResponse{}).
		Get("/sns/jscode2session")
	if err != nil {
		return nil, err
	}
	result, err := loadSuccessResponse[AuthResponse](resp, func(a *AuthResponse) error {
		return checkResponseError(a.ErrCode, a.ErrMsg)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (w *Wechat) GetAccessToken(ctx context.Context, reload bool) (string, error) {
	if !reload && w.accessToken != "" && time.Now().Before(w.accessTokenExpire) {
		return w.accessToken, nil
	}
	resp, err := w.client.R().
		Clone(ctx).
		SetQueryParams(map[string]string{
			"grant_type": "client_credential",
			"appid":      w.config.AppID,
			"secret":     w.config.AppSecret,
		}).
		SetResult(&AccessTokenResponse{}).
		SetError(&EmptyResponse{}).
		Get("/cgi-bin/token")
	if err != nil {
		return "", err
	}
	result, err := loadSuccessResponse[AccessTokenResponse](resp, func(a *AccessTokenResponse) error {
		return checkResponseError(a.ErrCode, a.ErrMsg)
	})
	if err != nil {
		return "", err
	}
	w.accessToken = result.AccessToken
	w.accessTokenExpire = time.Now().Add(time.Duration(result.ExpiresIn)*time.Second - 10*time.Second)
	return w.accessToken, nil
}

type RequestOptions struct {
	retryable         bool
	reloadAccessToken bool
}

type RequestOption func(*RequestOptions)

func WithRetryable(retryable bool) func(*RequestOptions) {
	return func(opts *RequestOptions) {
		opts.retryable = retryable
	}
}

func WithReloadAccessToken(reload bool) func(*RequestOptions) {
	return func(opts *RequestOptions) {
		opts.reloadAccessToken = reload
	}
}

func WithClone(opts *RequestOptions) func(*RequestOptions) {
	return func(o *RequestOptions) {
		o.retryable = opts.retryable
		o.reloadAccessToken = opts.reloadAccessToken
	}
}

func (w *Wechat) GetQrCode(ctx context.Context, code QrCodeRequest, options ...RequestOption) ([]byte, error) {
	opts := &RequestOptions{}
	for _, opt := range options {
		opt(opts)
	}
	token, err := w.GetAccessToken(ctx, opts.reloadAccessToken)
	if err != nil {
		return nil, err
	}
	if code.EnvVersion == "" {
		code.EnvVersion = w.config.Env.String()
	}
	resp, err := w.client.R().
		Clone(ctx).
		SetQueryParams(map[string]string{
			"access_token": token,
		}).
		SetBody(code).
		SetError(&EmptyResponse{}).
		Post("/wxa/getwxacodeunlimit")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() == 200 {
		return resp.Bytes(), nil
	}
	var result EmptyResponse
	err = json.Unmarshal(resp.Bytes(), &result)
	if err != nil {
		return nil, err
	}
	err = checkResponseError(result.ErrCode, result.ErrMsg)
	if err != nil {
		if opts.retryable && isNeedRetryError(err) {
			return w.GetQrCode(ctx, code,
				WithClone(opts),
				WithReloadAccessToken(true),
				WithRetryable(false),
			)
		}
		return nil, err
	}
	return nil, fmt.Errorf("get qr code error: %s", resp.Status())
}

func (w *Wechat) SendMessage(ctx context.Context, msg SubscribeMessageRequest, options ...RequestOption) error {
	opts := &RequestOptions{}
	for _, opt := range options {
		opt(opts)
	}
	token, err := w.GetAccessToken(ctx, opts.reloadAccessToken)
	if err != nil {
		return err
	}
	if msg.MiniprogramState == "" {
		switch w.config.Env {
		case "release":
			msg.MiniprogramState = "formal"
		case "trial":
			msg.MiniprogramState = "trial"
		case "develop":
			msg.MiniprogramState = "developer"
		}
	}
	resp, err := w.client.R().
		Clone(ctx).
		SetQueryParams(map[string]string{
			"access_token": token,
		}).
		SetBody(msg).
		SetResult(&EmptyResponse{}).
		SetError(&EmptyResponse{}).
		Post("/cgi-bin/message/subscribe/send")
	if err != nil {
		return err
	}
	_, err = loadSuccessResponse[EmptyResponse](resp, func(a *EmptyResponse) error {
		return checkResponseError(a.ErrCode, a.ErrMsg)
	})
	if err != nil {
		if opts.retryable && isNeedRetryError(err) {
			return w.SendMessage(ctx, msg,
				WithClone(opts),
				WithReloadAccessToken(true),
				WithRetryable(false),
			)
		}
		return err
	}
	return nil
}

func (w *Wechat) SendMessageWithTemplate(ctx context.Context, temp *PushTemplateConfig, values []any, toUser string) error {
	data := make(map[string]any, len(temp.TemplateKeys))
	for i, k := range temp.TemplateKeys {
		if i < len(values) {
			data[k] = map[string]any{"value": values[i]}
		}
	}
	msg := SubscribeMessageRequest{
		TemplateID:       temp.TemplateId,
		Page:             temp.Page,
		ToUser:           toUser,
		Data:             data,
		MiniprogramState: w.config.Env.String(),
		Lang:             "zh_CN",
	}
	return w.SendMessage(ctx, msg, WithRetryable(true))
}

func (w *Wechat) GetUserPhoneNumber(ctx context.Context, code string, options ...RequestOption) (*GetUserPhoneNumberResponse, error) {
	opts := &RequestOptions{}
	for _, opt := range options {
		opt(opts)
	}
	token, err := w.GetAccessToken(ctx, opts.reloadAccessToken)
	if err != nil {
		return nil, err
	}
	resp, err := w.client.R().
		Clone(ctx).
		SetQueryParams(map[string]string{
			"access_token": token,
		}).
		SetBody(map[string]string{"code": code}).
		SetResult(&GetUserPhoneNumberResponse{}).
		SetError(&EmptyResponse{}).
		Post("/wxa/business/getuserphonenumber")
	if err != nil {
		return nil, err
	}
	result, err := loadSuccessResponse[GetUserPhoneNumberResponse](resp, func(a *GetUserPhoneNumberResponse) error {
		return checkResponseError(a.ErrCode, a.ErrMsg)
	})
	if err != nil {
		if opts.retryable && isNeedRetryError(err) {
			return w.GetUserPhoneNumber(ctx, code,
				WithClone(opts),
				WithReloadAccessToken(true),
				WithRetryable(false),
			)
		}
		return nil, err
	}
	return result, nil
}

func TruncateString(s string, maxChars int) string {
	if maxChars <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxChars {
		return s
	}
	truncated := ""
	count := 0
	for _, runeValue := range s {
		if count >= maxChars {
			break
		}
		truncated += string(runeValue)
		count++
	}
	return truncated
}
