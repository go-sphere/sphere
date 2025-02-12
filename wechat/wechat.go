package wechat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/log/logfields"
	"github.com/TBXark/sphere/utils/httpclient"
	"io"
	"net/http"
	"time"
	"unicode/utf8"
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
}

func NewWechat(config *Config) *Wechat {
	if config.Env == "" {
		config.Env = "release"
	}
	return &Wechat{
		config: config,
	}
}

func (w *Wechat) Auth(code string) (*AuthResponse, error) {
	url, err := httpclient.URL("https://api.weixin.qq.com/sns/jscode2session", map[string]string{
		"appid":      w.config.AppID,
		"secret":     w.config.AppSecret,
		"js_code":    code,
		"grant_type": "authorization_code",
	})
	if err != nil {
		return nil, err
	}
	result, err := httpclient.GET[AuthResponse](url)
	if err != nil {
		return nil, err
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("auth error: %s", result.ErrMsg)
	}
	return result, nil
}

func (w *Wechat) GetAccessToken(reload bool) (string, error) {
	if !reload && w.accessToken != "" && time.Now().Before(w.accessTokenExpire) {
		return w.accessToken, nil
	}
	url, err := httpclient.URL("https://api.weixin.qq.com/cgi-bin/token", map[string]string{
		"grant_type": "client_credential",
		"appid":      w.config.AppID,
		"secret":     w.config.AppSecret,
	})
	if err != nil {
		return "", err
	}
	result, err := httpclient.GET[AccessTokenResponse](url)
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

func (w *Wechat) GetQrCode(code QrCodeRequest, retryable bool) ([]byte, error) {
	token, err := w.GetAccessToken(false)
	if err != nil {
		return nil, err
	}
	if code.EnvVersion == "" {
		code.EnvVersion = w.config.Env.String()
	}
	url, err := httpclient.URL("https://api.weixin.qq.com/wxa/getwxacodeunlimit", map[string]string{
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
	resp, err := httpclient.DefaultHttpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return io.ReadAll(resp.Body)
	}
	var result EmptyResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("%s", resp.Status)
	}
	if isNeedRetryErrorCode(result.ErrCode) && retryable {
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

func (w *Wechat) SendMessage(msg SubscribeMessageRequest, retryable bool) error {
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
	url, err := httpclient.URL("https://api.weixin.qq.com/cgi-bin/message/subscribe/send", map[string]string{
		"access_token": token,
	})
	if err != nil {
		return err
	}
	result, err := httpclient.POST[EmptyResponse](url, msg)
	if err != nil {
		return err
	}
	if result.ErrCode != 0 {
		if isNeedRetryErrorCode(result.ErrCode) && retryable {
			_, err = w.GetAccessToken(true)
			if err != nil {
				return err
			}
			return w.SendMessage(msg, false)
		}
		retErr := fmt.Errorf("send message error: %s", result.ErrMsg)
		log.Warnw(
			"send message error",
			logfields.Error(retErr),
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
	msg := SubscribeMessageRequest{
		TemplateID:       temp.TemplateId,
		Page:             temp.Page,
		ToUser:           toUser,
		Data:             data,
		MiniprogramState: w.config.Env.String(),
		Lang:             "zh_CN",
	}
	return w.SendMessage(msg, true)
}

func (w *Wechat) GetUserPhoneNumber(code string, retryable bool) (*GetUserPhoneNumberResponse, error) {
	token, err := w.GetAccessToken(false)
	if err != nil {
		return nil, err
	}
	url, err := httpclient.URL("https://api.weixin.qq.com/wxa/business/getuserphonenumber", map[string]string{
		"access_token": token,
	})
	if err != nil {
		return nil, err
	}
	result, err := httpclient.POST[GetUserPhoneNumberResponse](url, map[string]string{"code": code})
	if err != nil {
		return nil, err
	}
	if result.ErrCode != 0 {
		if isNeedRetryErrorCode(result.ErrCode) && retryable {
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
