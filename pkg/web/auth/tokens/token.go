package tokens

import (
	"fmt"
	"github.com/golang-jwt/jwt"
	"strings"
	"time"
)

type SignedDetails struct {
	UID      string `json:"uid"`
	Username string `json:"username"`
	Roles    string `json:"roles"`
	jwt.StandardClaims
}

type Generator struct {
	secretKey             []byte
	SignedTokenDuration   time.Duration
	SignedRefreshDuration time.Duration
}

func NewTokenGenerator(secretKey string, signedTokenDuration time.Duration, signedRefreshDuration time.Duration) *Generator {
	return &Generator{
		secretKey:             []byte(secretKey),
		SignedTokenDuration:   signedTokenDuration,
		SignedRefreshDuration: signedRefreshDuration,
	}
}

func (g *Generator) GenerateSignedToken(uid, username string, roles ...string) (string, error) {
	claims := &SignedDetails{
		UID:      uid,
		Username: username,
		Roles:    strings.Join(roles, ","),
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Local().Add(g.SignedTokenDuration).Unix(),
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(g.secretKey)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (g *Generator) GenerateRefreshToken() (string, error) {

	refreshClaims := &SignedDetails{
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Local().Add(g.SignedRefreshDuration).Unix(),
		},
	}

	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString(g.secretKey)
	if err != nil {
		return "", err
	}

	return refreshToken, nil
}

func (g *Generator) Validate(signedToken string) (claims *SignedDetails, err error) {
	token, err := jwt.ParseWithClaims(signedToken, &SignedDetails{}, func(token *jwt.Token) (interface{}, error) {
		return g.secretKey, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("token is invalid")
	}

	claims, ok := token.Claims.(*SignedDetails)
	if !ok {
		return nil, fmt.Errorf("token is invalid")
	}

	if claims.ExpiresAt < time.Now().Local().Unix() {
		return nil, fmt.Errorf("token is expired")
	}

	return claims, nil
}
