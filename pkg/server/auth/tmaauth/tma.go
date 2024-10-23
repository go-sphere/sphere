package tmaauth

import (
	"github.com/tbxark/sphere/pkg/server/auth/authorizer"
	initdata "github.com/telegram-mini-apps/init-data-golang"
	"time"
)

const AuthorizationPrefixTMA = "tma"

type Claims struct {
	UID       int64              `json:"uid"`
	InitData  *initdata.InitData `json:"init_data"`
	ExpiresAt time.Time          `json:"exp"`
}

var _ authorizer.Claims = &Claims{}

func (c *Claims) Valid() error {
	if c.ExpiresAt.Before(time.Now()) {
		return authorizer.ErrorExpiredToken
	}
	return nil
}

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

func (t *TmaAuth) ParseToken(token string) (*Claims, error) {
	err := initdata.Validate(token, t.token, t.expIn)
	if err != nil {
		return nil, err
	}
	initData, err := initdata.Parse(token)
	if err != nil {
		return nil, err
	}
	claims := Claims{
		UID:       initData.User.ID,
		InitData:  &initData,
		ExpiresAt: time.Now().Add(t.expIn),
	}
	return &claims, nil
}
