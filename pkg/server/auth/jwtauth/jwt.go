package jwtauth

import (
	"github.com/tbxark/sphere/pkg/server/auth/authorizer"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
)

const (
	AuthorizationPrefixBearer = "Bearer"
)

var _ authorizer.Authorizer[int64] = &JwtAuth[int64]{}

type SignedDetails[T authorizer.UID] struct {
	jwt.StandardClaims
	UID   T      `json:"uid,omitempty"`
	Roles string `json:"roles,omitempty"`
}

type JwtAuth[T authorizer.UID] struct {
	secret        []byte
	signingMethod jwt.SigningMethod
}

func NewJwtAuth[T authorizer.UID](secret string) *JwtAuth[T] {
	ja := &JwtAuth[T]{
		secret:        []byte(secret),
		signingMethod: jwt.SigningMethodHS256,
	}
	return ja
}

func (g *JwtAuth[T]) GenerateToken(claims *authorizer.Claims[T]) (*authorizer.Token, error) {

	token, err := jwt.NewWithClaims(g.signingMethod, claims).SignedString(g.secret)
	if err != nil {
		return nil, err
	}

	return &authorizer.Token{
		Token:     token,
		ExpiresAt: time.Unix(claims.ExpiresAt, 0),
	}, nil
}

func (g *JwtAuth[T]) ParseToken(signedToken string) (*authorizer.Claims[T], error) {
	claims := &authorizer.Claims[T]{}
	_, err := jwt.ParseWithClaims(signedToken, claims, func(token *jwt.Token) (interface{}, error) {
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

func (g *JwtAuth[T]) ParseRoles(roles string) []string {
	return strings.Split(roles, ",")
}

func (g *JwtAuth[T]) GenerateRoles(roles []string) string {
	return strings.Join(roles, ",")
}
