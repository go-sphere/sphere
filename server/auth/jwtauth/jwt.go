package jwtauth

import (
	"context"
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

func (g *JwtAuth[T]) GenerateToken(ctx context.Context, claims *T) (string, error) {
	if claims == nil {
		return "", fmt.Errorf("claims must not be nil")
	}
	token, err := jwt.NewWithClaims(g.signingMethod, *claims).SignedString(g.secret)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (g *JwtAuth[T]) ParseToken(ctx context.Context, signedToken string) (*T, error) {
	claims := new(T)
	// magically, if you parse *claims, it will panic "token is malformed: could not JSON decode claim: json: cannot unmarshal object into Go value of type jwt.Claims"
	jwtClaims, ok := any(claims).(jwt.Claims)
	if !ok {
		return nil, fmt.Errorf("claims must be jwt.Claims")
	}
	// ParseWithClaims second argument must be a pointer to jwt.Claims
	// Or you can do this:
	// var claims T
	// jwtClaims, ok := any(&claims).(jwt.Claims)
	_, err := jwt.ParseWithClaims(signedToken, jwtClaims, func(token *jwt.Token) (interface{}, error) {
		return g.secret, nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}
