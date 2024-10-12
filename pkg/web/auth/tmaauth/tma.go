package tmaauth

import (
	initdata "github.com/telegram-mini-apps/init-data-golang"
	"strconv"
	"time"
)

const AuthorizationPrefixTMA = "tma"

type TmaAuth struct {
	token string
	expIn time.Duration
}

func NewTmaAuth(token string) *TmaAuth {
	return &TmaAuth{
		token: token,
		expIn: time.Hour * 24,
	}
}

func (t *TmaAuth) Validate(token string) (uid string, username string, roles string, exp int64, err error) {
	err = initdata.Validate(token, t.token, t.expIn)
	if err != nil {
		return
	}
	initData, err := initdata.Parse(token)
	if err != nil {
		return
	}

	uid = strconv.Itoa(int(initData.Chat.ID))
	username = initData.Chat.Username
	roles = string(initData.Chat.Type)
	exp = initData.AuthDate().Add(t.expIn).Unix()
	return
}

func (t *TmaAuth) ParseRoles(roles string) map[string]struct{} {
	return make(map[string]struct{})
}
