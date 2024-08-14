package tma_tokens

import (
	initdata "github.com/telegram-mini-apps/init-data-golang"
	"time"
)

const AuthorizationPrefixTMA = "tma"

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

func (t *TmaAuth) Validate(token string) (map[string]any, error) {
	err := initdata.Validate(token, t.token, t.expIn)
	if err != nil {
		return nil, err
	}
	initData, err := initdata.Parse(token)
	if err != nil {
		return nil, err
	}

	res := make(map[string]any)
	res["uid"] = initData.Chat.ID
	res["username"] = initData.Chat.Username
	res["roles"] = ""
	res["exp"] = initData.AuthDate().Add(t.expIn)

	return res, nil
}

func (t *TmaAuth) ParseRolesString(roles string) map[string]struct{} {
	return make(map[string]struct{})
}
