package tmaauth

import (
	"github.com/tbxark/sphere/pkg/web/auth/parser"
	initdata "github.com/telegram-mini-apps/init-data-golang"
	"strconv"
	"time"
)

const AuthorizationPrefixTMA = "tma"

var _ parser.AuthParser = &TmaAuth{}

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

func (t *TmaAuth) ParseToken(token string) (*parser.Claims, error) {
	err := initdata.Validate(token, t.token, t.expIn)
	if err != nil {
		return nil, err
	}
	initData, err := initdata.Parse(token)
	if err != nil {
		return nil, err
	}
	return &parser.Claims{
		Subject:  strconv.Itoa(int(initData.Chat.ID)),
		Username: initData.Chat.Username,
		Roles:    string(initData.Chat.Type),
		Exp:      initData.AuthDate().Add(t.expIn).Unix(),
	}, nil
}

func (t *TmaAuth) ParseRoles(roles string) []string {
	return []string{roles}
}
