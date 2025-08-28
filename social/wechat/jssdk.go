package wechat

import (
	"context"
	"crypto/sha1"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-sphere/sphere/utils/encoding/numconv"
)

func (w *Wechat) SnsOauth2(ctx context.Context, code string) (*SnsOauth2Response, error) {
	resp, err := w.client.R().
		Clone(ctx).
		SetHeader("Accept", "application/json").
		SetQueryParams(map[string]string{
			"appid":      w.config.AppID,
			"secret":     w.config.AppSecret,
			"code":       code,
			"grant_type": "authorization_code",
		}).
		Get("/sns/oauth2/access_token")
	if err != nil {
		return nil, err
	}
	return loadSuccessResponse(resp, func(a *SnsOauth2Response) error {
		return nil
	})
}

func (w *Wechat) GetJsSDKConfig(ctx context.Context, url string) (*JsSDKConfigResponse, error) {
	ticket, err := w.GetJsTicket(ctx, false)
	if err != nil {
		return nil, err
	}
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := numconv.RandomBase62(16)
	params := map[string]string{
		"noncestr":     nonce,
		"jsapi_ticket": ticket,
		"timestamp":    timestamp,
		"url":          url,
	}
	signature := generateSignature(params)
	return &JsSDKConfigResponse{
		AppId:     w.config.AppID,
		Timestamp: timestamp,
		NonceStr:  nonce,
		Signature: signature,
	}, nil
}

func generateSignature(params map[string]string) string {
	var pairs []string
	for k, v := range params {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(pairs)
	string1 := strings.Join(pairs, "&")
	hash := sha1.New()
	hash.Write([]byte(string1))
	return fmt.Sprintf("%x", hash.Sum(nil))
}
