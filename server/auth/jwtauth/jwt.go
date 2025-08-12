package jwtauth

import (
	"context"
	"encoding/json"
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
	var claims T
	// Although the second parameter in jwt.ParseWithClaims requires a jwt.Claims type,
	// when claims is a struct type, directly passing it for parsing will result in the following error:
	// > token is malformed: could not JSON decode claim: json: cannot unmarshal object into Go value of type jwt.Claims
	// Therefore, you must pass a pointer to claims, and also ensure that *T is of type jwt.Claims.
	if jwtClaims, ok := any(&claims).(jwt.Claims); ok {
		_, err := jwt.ParseWithClaims(signedToken, jwtClaims, func(token *jwt.Token) (interface{}, error) {
			return g.secret, nil
		})
		if err != nil {
			return nil, err
		}
		return &claims, nil
	} else {
		// Otherwise, first parse it into a map, then attempt to convert it into T.
		token, err := jwt.Parse(signedToken, func(token *jwt.Token) (interface{}, error) {
			return g.secret, nil
		})
		if err != nil {
			return nil, err
		}
		// Here, mapstructure cannot be used because it has issues with converting anonymous fields.
		raw, err := json.Marshal(token.Claims)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(raw, &claims)
		if err != nil {
			return nil, err
		}
		return &claims, nil
	}
}
