package jwtauth

import (
	"fmt"
	"github.com/golang-jwt/jwt"
	"strings"
	"time"
)

const (
	AuthorizationPrefixBearer = "Bearer"
)

type SignedDetails struct {
	UID      string `json:"uid"`
	Username string `json:"username"`
	Roles    string `json:"roles"`
	jwt.StandardClaims
}

type Token struct {
	Token     string
	ExpiresAt time.Time
}

type JwtAuth struct {
	secretKey             []byte
	SignedTokenDuration   time.Duration
	SignedRefreshDuration time.Duration
}

func NewJwtAuth(secretKey string) *JwtAuth {
	return &JwtAuth{
		secretKey:             []byte(secretKey),
		SignedTokenDuration:   time.Hour * 24,
		SignedRefreshDuration: time.Hour * 24 * 7,
	}
}

func (g *JwtAuth) GenerateSignedToken(uid, username string, roles ...string) (*Token, error) {
	expiresAt := time.Now().Local().Add(g.SignedTokenDuration)
	claims := &SignedDetails{
		UID:      uid,
		Username: username,
		Roles:    strings.Join(roles, ","),
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expiresAt.Unix(),
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(g.secretKey)
	if err != nil {
		return nil, err
	}

	return &Token{
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

func (g *JwtAuth) GenerateRefreshToken(uid string) (*Token, error) {

	expiresAt := time.Now().Local().Add(g.SignedRefreshDuration)
	refreshClaims := &SignedDetails{
		UID: uid,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expiresAt.Unix(),
		},
	}

	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString(g.secretKey)
	if err != nil {
		return nil, err
	}

	return &Token{
		Token:     refreshToken,
		ExpiresAt: expiresAt,
	}, nil
}

func (g *JwtAuth) Validate(signedToken string) (uid string, username string, roles string, exp int64, err error) {
	token, err := jwt.ParseWithClaims(signedToken, &SignedDetails{}, func(token *jwt.Token) (interface{}, error) {
		return g.secretKey, nil
	})
	if err != nil {
		return
	}

	claims, ok := token.Claims.(*SignedDetails)
	if !ok {
		err = fmt.Errorf("token is invalid")
		return
	}
	uid = claims.UID
	username = claims.Username
	roles = claims.Roles
	exp = claims.ExpiresAt
	return
}

func (g *JwtAuth) ParseRoles(roles string) map[string]struct{} {
	roleMap := make(map[string]struct{})
	if roles == "" {
		return roleMap
	}
	for _, r := range strings.Split(roles, ",") {
		roleMap[r] = struct{}{}
	}
	return roleMap
}
