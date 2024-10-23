package tmaauth

import (
	"github.com/tbxark/sphere/pkg/server/auth/authorizer"
	initdata "github.com/telegram-mini-apps/init-data-golang"
	"time"
)

const AuthorizationPrefixTMA = "tma"

var _ authorizer.Parser[int64] = &TmaAuth{}

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

func (t *TmaAuth) ParseToken(token string) (*authorizer.Claims[int64], error) {
	err := initdata.Validate(token, t.token, t.expIn)
	if err != nil {
		return nil, err
	}
	initData, err := initdata.Parse(token)
	if err != nil {
		return nil, err
	}
	return &authorizer.Claims[int64]{
		UID:     initData.Chat.ID,
		Subject: initData.Chat.Username,
		Roles:   string(initData.Chat.Type),
		Exp:     initData.AuthDate().Add(t.expIn).Unix(),
	}, nil
}

func (t *TmaAuth) ParseRoles(roles string) []string {
	return []string{roles}
}
