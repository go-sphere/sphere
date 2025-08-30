package wechat

import (
	"encoding/json"
	"errors"
	"fmt"

	"resty.dev/v3"
)

const (
	// ErrCodeInvalidCredential 40001 获取access_token时AppSecret错误，或者access_token无效
	ErrCodeInvalidCredential = 40001
	// ErrCodeAccessTokenExpired 42001 access_token超时
	ErrCodeAccessTokenExpired = 42001
	// ErrCodeInvalidAccessToken 40014 不合法的access_token，请开发者认真比对access_token的有效性（如是否过期），或查看是否正在为恰当的公众号调用接口
	ErrCodeInvalidAccessToken = 40014
)

var (
	ErrorInvalidCredential  = errors.New("invalid credential")
	ErrorAccessTokenExpired = errors.New("access token expired")
	ErrorInvalidAccessToken = errors.New("invalid access token")
)

// isNeedRetryError determines if an error is recoverable and should trigger a retry.
// It checks for credential and access token errors that can be resolved by refreshing tokens.
func isNeedRetryError(err error) bool {
	if errors.Is(err, ErrorInvalidCredential) {
		return true
	}
	if errors.Is(err, ErrorAccessTokenExpired) {
		return true
	}
	if errors.Is(err, ErrorInvalidAccessToken) {
		return true
	}
	return false
}

// checkResponseError converts WeChat API error codes to Go errors.
// It maps common error codes to predefined error instances for consistent error handling.
func checkResponseError(errCode int, errMsg string) error {
	switch errCode {
	case ErrCodeInvalidCredential:
		return ErrorInvalidCredential
	case ErrCodeAccessTokenExpired:
		return ErrorAccessTokenExpired
	case ErrCodeInvalidAccessToken:
		return ErrorInvalidAccessToken
	}
	if errCode != 0 {
		return ErrResponse{
			ErrCode: errCode,
			ErrMsg:  errMsg,
		}
	}
	return nil
}

func loadSuccessResponse[T any](resp *resty.Response, check func(*T) error) (*T, error) {
	if resp.IsError() {
		var result ErrResponse
		err := json.Unmarshal(resp.Bytes(), &result)
		if err != nil {
			return nil, err
		}
		return nil, result
	}
	if resp.IsSuccess() {
		var result T
		err := json.Unmarshal(resp.Bytes(), &result)
		if err != nil {
			return nil, err
		}
		err = check(&result)
		if err != nil {
			return nil, err
		}
		return &result, nil
	}
	return nil, fmt.Errorf("unknown error: %s", resp.Status())
}

type ErrResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func (e ErrResponse) Error() string {
	return e.ErrMsg
}

type JsCode2SessionResponse struct {
	ErrResponse
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
}

type SnsOauth2Response struct {
	AccessToken    string `json:"access_token"`
	ExpiresIn      int    `json:"expires_in"`
	RefreshToken   string `json:"refresh_token"`
	OpenID         string `json:"openid"`
	Scope          string `json:"scope"`
	IsSnapshotUser int    `json:"is_snapshotuser"`
	UnionID        string `json:"unionid"`
}

type AccessTokenResponse struct {
	ErrResponse
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type JsTicketResponse struct {
	ErrResponse
	Ticket    string `json:"ticket"`
	ExpiresIn int    `json:"expires_in"`
}

type QrCodeRequest struct {
	Scene      string `json:"scene,omitempty"`       // 最大32个可见字符，只支持数字，大小写英文以及部分特殊字符：!#$&'()*+,/:;=?@-._~，其它字符请自行编码为合法字符（因不支持%，中文无法使用 urlencode 处理，请使用其他编码方式）
	Page       string `json:"page,omitempty"`        // 默认是主页，页面 page，例如 pages/index/index，根路径前不要填加 /，不能携带参数（参数请放在scene字段里），如果不填写这个字段，默认跳主页面。scancode_time为系统保留参数，不允许配置
	CheckPath  bool   `json:"check_path,omitempty"`  // 默认是true，检查page 是否存在，为 true 时 page 必须是已经发布的小程序存在的页面（否则报错）；为 false 时允许小程序未发布或者 page 不存在， 但page 有数量上限（60000个）请勿滥用。
	EnvVersion string `json:"env_version,omitempty"` // 要打开的小程序版本。正式版为 "release"，体验版为 "trial"，开发版为 "develop"。默认是正式版。
	Width      int    `json:"width,omitempty"`       // 默认430，二维码的宽度，单位 px，最小 280px，最大 1280px
	AutoColor  bool   `json:"auto_color,omitempty"`  // 自动配置线条颜色，如果颜色依然是黑色，则说明不建议配置主色调，默认 false
	LineColor  string `json:"line_color,omitempty"`  // 默认是{"r":0,"g":0,"b":0} 。auto_color 为 false 时生效，使用 rgb 设置颜色 例如 {"r":"xxx","g":"xxx","b":"xxx"} 十进制表示
	IsHyaline  bool   `json:"is_hyaline,omitempty"`  // 默认是false，是否需要透明底色，为 true 时，生成透明底色的小程序
}

type PushTemplateConfig struct {
	TemplateId   string   `json:"template_id"`
	TemplateNo   int      `json:"template_no"`
	TemplateKeys []string `json:"template_keys"`
	Page         string   `json:"page"`
}

type SubscribeMessageRequest struct {
	TemplateID       string         `json:"template_id"`       // 所需下发的订阅模板id
	Page             string         `json:"page"`              // 点击模板卡片后的跳转页面，仅限本小程序内的页面。支持带参数,（示例index?foo=bar）。该字段不填则模板无跳转
	ToUser           string         `json:"touser"`            // 接收者（用户）的 openid
	Data             map[string]any `json:"data"`              // 模板内容，格式形如 { "key1": { "value": any }, "key2": { "value": any } }的object
	MiniProgramState string         `json:"miniprogram_state"` // developer(开发版)、trial(体验版)、formal(正式版)
	Lang             string         `json:"lang"`              // zh_CN(简体中文)、en_US(英文)、zh_HK(繁体中文)、zh_TW(繁体中文)
}

type GetUserPhoneNumberResponse struct {
	ErrResponse
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

type JsSDKConfigResponse struct {
	AppId     string `json:"appId"`
	Timestamp string `json:"timestamp"`
	NonceStr  string `json:"nonceStr"`
	Signature string `json:"signature"`
}
