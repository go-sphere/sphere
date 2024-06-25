package wechat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/tbxark/go-base-api/pkg/log"
	"github.com/tbxark/go-base-api/pkg/log/field"
	"github.com/tbxark/go-base-api/pkg/request"
	"io"
	"net/http"
	"time"
	"unicode/utf8"
)

const (
	// ErrorInvalidCredential 40001 获取access_token时AppSecret错误，或者access_token无效
	ErrorInvalidCredential = 40001
	// ErrorAccessTokenExpired 42001 access_token超时
	ErrorAccessTokenExpired = 42001
	// ErrorInvalidAccessToken 40014 不合法的access_token，请开发者认真比对access_token的有效性（如是否过期），或查看是否正在为恰当的公众号调用接口
	ErrorInvalidAccessToken = 40014
)

func errorCodeNeedRetry(errCode int) bool {
	return errCode == ErrorAccessTokenExpired || errCode == ErrorInvalidAccessToken || errCode == ErrorInvalidCredential
}

type Config struct {
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
	Env       string `json:"env"` // 正式版为 "release"，体验版为 "trial"，开发版为 "develop"。默认是正式版。
}

type Wechat struct {
	config            *Config
	accessToken       string
	accessTokenExpire time.Time
}

func NewWechat(config *Config) *Wechat {
	if config.Env == "" {
		config.Env = "release"
	}
	return &Wechat{
		config: config,
	}
}

type wxMiniEmptyResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

type WxMiniAuthResponse struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

func (w *Wechat) Auth(code string) (*WxMiniAuthResponse, error) {
	url, err := request.URL("https://api.weixin.qq.com/sns/jscode2session", map[string]string{
		"appid":      w.config.AppID,
		"secret":     w.config.AppSecret,
		"js_code":    code,
		"grant_type": "authorization_code",
	})
	if err != nil {
		return nil, err
	}
	result, err := request.GET[WxMiniAuthResponse](url)
	if err != nil {
		return nil, err
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("auth error: %s", result.ErrMsg)
	}
	return result, nil
}

type wxMiniAccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

func (w *Wechat) GetAccessToken(reload bool) (string, error) {
	if !reload && w.accessToken != "" && time.Now().Before(w.accessTokenExpire) {
		return w.accessToken, nil
	}
	url, err := request.URL("https://api.weixin.qq.com/cgi-bin/token", map[string]string{
		"grant_type": "client_credential",
		"appid":      w.config.AppID,
		"secret":     w.config.AppSecret,
	})
	result, err := request.GET[wxMiniAccessTokenResponse](url)
	if err != nil {
		return "", err
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("get access token error: %s", result.ErrMsg)
	}
	w.accessToken = result.AccessToken
	w.accessTokenExpire = time.Now().Add(time.Duration(result.ExpiresIn)*time.Second - 10*time.Second)
	return w.accessToken, nil
}

type WxMiniQrCodeRequest struct {
	Scene      string `json:"scene,omitempty"`       // 最大32个可见字符，只支持数字，大小写英文以及部分特殊字符：!#$&'()*+,/:;=?@-._~，其它字符请自行编码为合法字符（因不支持%，中文无法使用 urlencode 处理，请使用其他编码方式）
	Page       string `json:"page,omitempty"`        // 默认是主页，页面 page，例如 pages/index/index，根路径前不要填加 /，不能携带参数（参数请放在scene字段里），如果不填写这个字段，默认跳主页面。scancode_time为系统保留参数，不允许配置
	CheckPath  bool   `json:"check_path,omitempty"`  // 默认是true，检查page 是否存在，为 true 时 page 必须是已经发布的小程序存在的页面（否则报错）；为 false 时允许小程序未发布或者 page 不存在， 但page 有数量上限（60000个）请勿滥用。
	EnvVersion string `json:"env_version,omitempty"` // 要打开的小程序版本。正式版为 "release"，体验版为 "trial"，开发版为 "develop"。默认是正式版。
	Width      int    `json:"width,omitempty"`       // 默认430，二维码的宽度，单位 px，最小 280px，最大 1280px
	AutoColor  bool   `json:"auto_color,omitempty"`  // 自动配置线条颜色，如果颜色依然是黑色，则说明不建议配置主色调，默认 false
	LineColor  string `json:"line_color,omitempty"`  // 默认是{"r":0,"g":0,"b":0} 。auto_color 为 false 时生效，使用 rgb 设置颜色 例如 {"r":"xxx","g":"xxx","b":"xxx"} 十进制表示
	IsHyaline  bool   `json:"is_hyaline,omitempty"`  // 默认是false，是否需要透明底色，为 true 时，生成透明底色的小程序
}

func (w *Wechat) GetQrCode(code WxMiniQrCodeRequest, retryable bool) ([]byte, error) {
	token, err := w.GetAccessToken(false)
	if err != nil {
		return nil, err
	}
	if code.EnvVersion == "" {
		code.EnvVersion = w.config.Env
	}
	url, err := request.URL("https://api.weixin.qq.com/wxa/getwxacodeunlimit", map[string]string{
		"access_token": token,
	})
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(code)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json; charset=utf-8")
	resp, err := request.DefaultHttpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return io.ReadAll(resp.Body)
	}
	var result wxMiniEmptyResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("%s", resp.Status)
	}
	if errorCodeNeedRetry(result.ErrCode) && retryable {
		_, err = w.GetAccessToken(true)
		if err != nil {
			return nil, err
		}
		return w.GetQrCode(code, false)
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("get qr code error: %s", result.ErrMsg)
	}
	return nil, fmt.Errorf("error: %s", resp.Status)
}

type PushTemplateConfig struct {
	TemplateId   string   `json:"template_id"`
	TemplateNo   int      `json:"template_no"`
	TemplateKeys []string `json:"template_keys"`
	Page         string   `json:"page"`
}

type WxMiniSubscribeMessageRequest struct {
	TemplateID       string         `json:"template_id"`       // 所需下发的订阅模板id
	Page             string         `json:"page"`              // 点击模板卡片后的跳转页面，仅限本小程序内的页面。支持带参数,（示例index?foo=bar）。该字段不填则模板无跳转
	ToUser           string         `json:"touser"`            // 接收者（用户）的 openid
	Data             map[string]any `json:"data"`              // 模板内容，格式形如 { "key1": { "value": any }, "key2": { "value": any } }的object
	MiniprogramState string         `json:"miniprogram_state"` // developer(开发版)、trial(体验版)、formal(正式版)
	Lang             string         `json:"lang"`              // zh_CN(简体中文)、en_US(英文)、zh_HK(繁体中文)、zh_TW(繁体中文)
}

func (w *Wechat) SendMessage(msg WxMiniSubscribeMessageRequest, retryable bool) error {
	token, err := w.GetAccessToken(false)
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
	url, err := request.URL("https://api.weixin.qq.com/cgi-bin/message/subscribe/send", map[string]string{
		"access_token": token,
	})
	result, err := request.POST[wxMiniEmptyResponse](url, msg)
	if err != nil {
		return err
	}
	if result.ErrCode != 0 {
		if errorCodeNeedRetry(result.ErrCode) && retryable {
			_, err = w.GetAccessToken(true)
			if err != nil {
				return err
			}
			return w.SendMessage(msg, false)
		}
		retErr := fmt.Errorf("send message error: %s", result.ErrMsg)
		log.Warnw(
			"send message error",
			field.Error(retErr),
		)
		return retErr
	}
	return nil
}

func (w *Wechat) SendMessageWithTemplate(temp *PushTemplateConfig, values []any, toUser string) error {
	data := make(map[string]any, len(temp.TemplateKeys))
	for i, k := range temp.TemplateKeys {
		if i < len(values) {
			data[k] = map[string]any{"value": values[i]}
		}
	}
	msg := WxMiniSubscribeMessageRequest{
		TemplateID:       temp.TemplateId,
		Page:             temp.Page,
		ToUser:           toUser,
		Data:             data,
		MiniprogramState: w.config.Env,
		Lang:             "zh_CN",
	}
	return w.SendMessage(msg, true)
}

type WxMiniGetUserPhoneNumberResponse struct {
	ErrCode   int    `json:"errcode"`
	ErrMsg    string `json:"errmsg"`
	PhoneInfo struct {
		PhoneNumber     string `json:"phoneNumber"`
		PurePhoneNumber string `json:"purePhoneNumber"`
		CountryCode     string `json:"countryCode"`
		Watermark       struct {
			Timestamp int    `json:"timestamp"`
			Appid     string `json:"appid"`
		} `json:"watermark"`
	} `json:"phone_info"`
}

func (w *Wechat) GetUserPhoneNumber(code string, retryable bool) (*WxMiniGetUserPhoneNumberResponse, error) {
	token, err := w.GetAccessToken(false)
	if err != nil {
		return nil, err
	}
	url, err := request.URL("https://api.weixin.qq.com/wxa/business/getuserphonenumber", map[string]string{
		"access_token": token,
	})
	if err != nil {
		return nil, err
	}
	result, err := request.POST[WxMiniGetUserPhoneNumberResponse](url, map[string]string{"code": code})
	if err != nil {
		return nil, err
	}
	if result.ErrCode != 0 {
		if errorCodeNeedRetry(result.ErrCode) && retryable {
			_, err = w.GetAccessToken(true)
			if err != nil {
				return nil, err
			}
			return w.GetUserPhoneNumber(code, false)
		}
		return nil, fmt.Errorf("get user phone number error: %s", result.ErrMsg)
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
