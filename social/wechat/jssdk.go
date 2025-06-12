package wechat

import "context"

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
