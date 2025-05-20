package tmaauth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"time"

	initdata "github.com/telegram-mini-apps/init-data-golang"
)

const AuthorizationPrefixTMA = "tma"

type Claims initdata.InitData

type TmaAuth struct {
	token string
	expIn time.Duration
}

func NewTmaAuth(token string, expIn time.Duration) *TmaAuth {
	return &TmaAuth{
		token: token,
		expIn: expIn,
	}
}

func (t *TmaAuth) ParseToken(ctx context.Context, token string) (*Claims, error) {
	err := initdata.Validate(token, t.token, t.expIn)
	if err != nil {
		return nil, err
	}
	data, err := initdata.Parse(token)
	if err != nil {
		return nil, err
	}
	return (*Claims)(&data), nil
}

func (t *TmaAuth) GenerateToken(ctx context.Context, claims *Claims) (string, error) {
	rawInitMap := map[string]any{}
	initBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	decoder := json.NewDecoder(bytes.NewReader(initBytes))
	decoder.UseNumber()
	err = decoder.Decode(&rawInitMap)
	if err != nil {
		return "", err
	}
	delete(rawInitMap, "hash")
	delete(rawInitMap, "auth_date")
	params := make(map[string]string)
	values := url.Values{}
	for k, v := range rawInitMap {
		if str, ok := v.(string); ok {
			params[k] = v.(string)
			values.Add(k, str)
			continue
		}
		if num, ok := v.(json.Number); ok {
			params[k] = num.String()
			values.Add(k, num.String())
			continue
		}
		partBytes, e := json.Marshal(v)
		if e != nil {
			return "", e
		}
		params[k] = string(partBytes)
		values.Add(k, string(partBytes))
	}
	exp := time.Unix(int64(claims.AuthDateRaw), 0)
	sign := initdata.Sign(params, t.token, exp)
	values.Set("hash", sign)
	values.Set("auth_date", strconv.FormatInt(exp.Unix(), 10))
	return values.Encode(), nil
}
