package jwtauth

import (
	"github.com/tbxark/sphere/pkg/server/auth/authorizer"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt"
)

const (
	AuthorizationPrefixBearer = "Bearer"
	DefaultTokenDuration      = time.Hour * 24 * 7
)

var _ authorizer.Authorizer[int64] = &JwtAuth[int64]{}

type SignedDetails[T authorizer.UID] struct {
	jwt.StandardClaims
	UID   T      `json:"uid,omitempty"`
	Roles string `json:"roles,omitempty"`
}

type JwtAuth[T authorizer.UID] struct {
	secret              []byte
	signingMethod       jwt.SigningMethod
	signedTokenDuration time.Duration
	mu                  sync.RWMutex // 添加互斥锁
}

type Option[T authorizer.UID] func(*JwtAuth[T])

func NewJwtAuth[T authorizer.UID](secret string, opts ...Option[T]) *JwtAuth[T] {
	ja := &JwtAuth[T]{
		secret:              []byte(secret),
		signingMethod:       jwt.SigningMethodHS256,
		signedTokenDuration: DefaultTokenDuration,
	}
	for _, opt := range opts {
		opt(ja)
	}
	return ja
}

func WithTokenDuration[T authorizer.UID](d time.Duration) Option[T] {
	return func(ja *JwtAuth[T]) {
		ja.signedTokenDuration = d
	}
}

func (g *JwtAuth[T]) GenerateToken(uid T, subject string, roles ...string) (*authorizer.Token, error) {
	expiresAt := time.Now().Local().Add(g.signedTokenDuration)
	claims := &SignedDetails[T]{
		UID:   uid,
		Roles: strings.Join(roles, ","),
		StandardClaims: jwt.StandardClaims{
			Subject:   subject,
			ExpiresAt: expiresAt.Unix(),
		},
	}

	token, err := jwt.NewWithClaims(g.signingMethod, claims).SignedString(g.secret)
	if err != nil {
		return nil, err
	}

	return &authorizer.Token{
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

func (g *JwtAuth[T]) ParseToken(signedToken string) (*authorizer.Claims[T], error) {
	claims := &SignedDetails[T]{}
	_, err := jwt.ParseWithClaims(signedToken, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return g.secret, nil
	})
	if err != nil {
		return nil, err
	}
	return &authorizer.Claims[T]{
		UID:     claims.UID,
		Subject: claims.Subject,
		Roles:   claims.Roles,
		Exp:     claims.ExpiresAt,
	}, nil
}

func (g *JwtAuth[T]) ParseRoles(roles string) []string {
	return strings.Split(roles, ",")
}

func (g *JwtAuth[T]) SetTokenDuration(duration time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.signedTokenDuration = duration
}
