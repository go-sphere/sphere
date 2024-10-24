package jwtauth

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
)

const (
	AuthorizationPrefixBearer = "Bearer"
)

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
	jwtClaims, ok := any(claims).(jwt.Claims) // magic, if you parse *claims, it will panic "token is malformed: could not JSON decode claim: json: cannot unmarshal object into Go value of type jwt.Claims"
	if !ok {
		return nil, fmt.Errorf("claims must be jwt.Claims")
	}
	token, err := jwt.ParseWithClaims(signedToken, jwtClaims, func(token *jwt.Token) (interface{}, error) {
		if _, mOk := token.Method.(*jwt.SigningMethodHMAC); !mOk {
			return nil, jwt.ErrSignatureInvalid
		}
		return g.secret, nil
	})
	if err != nil {
		return nil, err
	}
	res, ok := any(token.Claims).(*T)
	if !ok {
		return nil, fmt.Errorf("claims must be jwt.Claims")
	}
	return res, nil
}
