package jwtauth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt"
)

const (
	AuthorizationPrefixBearer = "Bearer"
)

type Jwt struct {
	token   string
	expires time.Time
}

func (t *Jwt) String() string {
	return t.token
}

func (t *Jwt) ExpiresAt() time.Time {
	return t.expires
}

type JwtAuth[T jwt.Claims] struct {
	secret        []byte
	signingMethod jwt.SigningMethod
}

func NewJwtAuth[T jwt.Claims](secret string) *JwtAuth[T] {
	ja := &JwtAuth[T]{
		secret:        []byte(secret),
		signingMethod: jwt.SigningMethodHS256,
	}
	return ja
}

func (g *JwtAuth[T]) GenerateToken(claims *T) (string, error) {
	token, err := jwt.NewWithClaims(g.signingMethod, *claims).SignedString(g.secret)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (g *JwtAuth[T]) ParseToken(signedToken string) (*T, error) {
	claims := new(T)
	jwtClaims, ok := any(claims).(jwt.Claims) // magic
	if !ok {
		return nil, fmt.Errorf("claims must be jwt.Claims")
	}
	_, err := jwt.ParseWithClaims(signedToken, jwtClaims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return g.secret, nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}
