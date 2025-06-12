package wechat

import (
	"context"
	"encoding/json"
)

func (w *Wechat) JsCode2Session(ctx context.Context, code string) (*JsCode2SessionResponse, error) {
	resp, err := w.client.R().
		Clone(ctx).
		SetHeader("Accept", "application/json").
		SetQueryParams(map[string]string{
			"appid":      w.config.AppID,
			"secret":     w.config.AppSecret,
			"js_code":    code,
			"grant_type": "authorization_code",
		}).
		Get("/sns/jscode2session")
	if err != nil {
		return nil, err
	}
	return loadSuccessResponse(resp, func(a *JsCode2SessionResponse) error {
		return checkResponseError(a.ErrCode, a.ErrMsg)
	})
}

func (w *Wechat) GetQrCode(ctx context.Context, code *QrCodeRequest, options ...RequestOption) ([]byte, error) {
	type Image struct {
		raw []byte
	}
	image, err := withAccessToken[Image](ctx, w, func(ctx context.Context, accessToken string) (*Image, error) {
		resp, err := w.client.R().
			Clone(ctx).
			SetQueryParams(map[string]string{
				"access_token": accessToken,
			}).
			SetBody(code).
			Post("/wxa/getwxacodeunlimit")
		if err != nil {
			return nil, err
		}
		res := resp.Bytes()
		if resp.StatusCode() == 200 {
			return &Image{raw: res}, nil
		}
		var errResp ErrResponse
		err = json.Unmarshal(res, &errResp)
		if err != nil {
			return nil, err
		}
		return nil, &errResp
	}, options...)
	if err != nil {
		return nil, err
	}
	return image.raw, nil
}

func (w *Wechat) SendMessage(ctx context.Context, msg *SubscribeMessageRequest, options ...RequestOption) error {
	_, err := withAccessToken(ctx, w, func(ctx context.Context, accessToken string) (*ErrResponse, error) {
		if msg.MiniProgramState == "" {
			switch w.config.Env {
			case "release":
				msg.MiniProgramState = "formal"
			case "trial":
				msg.MiniProgramState = "trial"
			case "develop":
				msg.MiniProgramState = "developer"
			}
		}
		resp, err := w.client.R().
			Clone(ctx).
			SetQueryParams(map[string]string{
				"access_token": accessToken,
			}).
			SetBody(msg).
			Post("/cgi-bin/message/subscribe/send")
		if err != nil {
			return nil, err
		}
		_, err = loadSuccessResponse(resp, func(a *ErrResponse) error {
			if a.ErrCode != 0 {
				return checkResponseError(a.ErrCode, a.ErrMsg)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		return nil, nil
	})
	return err
}

func (w *Wechat) GetUserPhoneNumber(ctx context.Context, code string, options ...RequestOption) (*GetUserPhoneNumberResponse, error) {
	return withAccessToken(ctx, w, func(ctx context.Context, accessToken string) (*GetUserPhoneNumberResponse, error) {
		resp, err := w.client.R().
			Clone(ctx).
			SetQueryParams(map[string]string{
				"access_token": accessToken,
			}).
			SetBody(map[string]string{"code": code}).
			SetResult(GetUserPhoneNumberResponse{}).
			SetError(ErrResponse{}).
			Post("/wxa/business/getuserphonenumber")
		if err != nil {
			return nil, err
		}
		return loadSuccessResponse(resp, func(a *GetUserPhoneNumberResponse) error {
			return checkResponseError(a.ErrCode, a.ErrMsg)
		})
	}, options...)
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
		MiniProgramState: w.config.Env.String(),
		Lang:             "zh_CN",
	}
	return w.SendMessage(ctx, &msg, WithRetryable(true))
}
