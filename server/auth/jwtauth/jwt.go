package jwtauth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

type options struct {
	signingMethod jwt.SigningMethod
}

// Option is a functional option for configuring JWT authentication.
type Option func(*options)

// WithSigningMethod sets the JWT signing method for token generation and verification.
func WithSigningMethod(method jwt.SigningMethod) Option {
	return func(opts *options) {
		opts.signingMethod = method
	}
}

func newOptions(opts ...Option) options {
	defaults := options{
		signingMethod: jwt.SigningMethodHS256,
	}
	for _, opt := range opts {
		opt(&defaults)
	}
	return defaults
}

// JwtAuth provides JWT token generation and verification functionality.
// It is parameterized by the claims type for type safety.
type JwtAuth[T jwt.Claims] struct {
	secret        []byte
	signingMethod jwt.SigningMethod
}

// NewJwtAuth creates a new JWT authenticator with the specified secret and options.
// The default signing method is HMAC-SHA256.
func NewJwtAuth[T jwt.Claims](secret string, options ...Option) *JwtAuth[T] {
	opts := newOptions(options...)
	ja := &JwtAuth[T]{
		secret:        []byte(secret),
		signingMethod: opts.signingMethod,
	}
	return ja
}

// keyFunc validates the token's signing method and returns the secret key for verification.
func (g *JwtAuth[T]) keyFunc(token *jwt.Token) (interface{}, error) {
	if token.Method.Alg() != g.signingMethod.Alg() {
		return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
	}
	return g.secret, nil
}

// GenerateToken creates a signed JWT token from the provided claims.
// The claims parameter must not be nil.
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

// ParseToken parses and validates a signed JWT token, returning the claims.
// It handles both direct jwt.Claims types and custom structs, using JSON
// marshaling/unmarshaling for struct conversion when necessary.
func (g *JwtAuth[T]) ParseToken(ctx context.Context, signedToken string) (*T, error) {
	var claims T
	// Although the second parameter in jwt.ParseWithClaims requires a jwt.Claims type,
	// when claims is a struct type, directly passing it for parsing will result in the following error:
	// > token is malformed: could not JSON decode claim: json: cannot unmarshal object into Go value of type jwt.Claims
	// Therefore, you must pass a pointer to claims, and also ensure that *T is of type jwt.Claims.
	if jwtClaims, ok := any(&claims).(jwt.Claims); ok {
		_, err := jwt.ParseWithClaims(signedToken, jwtClaims, g.keyFunc)
		if err != nil {
			return nil, err
		}
		return &claims, nil
	} else {
		// Otherwise, first parse it into a map, then attempt to convert it into T.
		token, err := jwt.Parse(signedToken, g.keyFunc)
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
