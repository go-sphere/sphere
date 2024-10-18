package jwtauth

import (
	"github.com/tbxark/sphere/pkg/web/auth/authorizer"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt"
)

const (
	AuthorizationPrefixBearer = "Bearer"
	DefaultTokenDuration      = time.Hour * 24 * 7
	DefaultRefreshDuration    = time.Hour * 24 * 7
)

var _ authorizer.Authorizer = &JwtAuth{}

type SignedDetails struct {
	jwt.StandardClaims
	Username string `json:"username"`
	Roles    string `json:"roles"`
}

type JwtAuth struct {
	secret                []byte
	signingMethod         jwt.SigningMethod
	signedTokenDuration   time.Duration
	signedRefreshDuration time.Duration
	mu                    sync.RWMutex // 添加互斥锁
}

type Option func(*JwtAuth)

func NewJwtAuth(secret string, opts ...Option) *JwtAuth {
	ja := &JwtAuth{
		secret:                []byte(secret),
		signingMethod:         jwt.SigningMethodHS256,
		signedTokenDuration:   DefaultTokenDuration,
		signedRefreshDuration: DefaultRefreshDuration,
	}
	for _, opt := range opts {
		opt(ja)
	}
	return ja
}

func WithTokenDuration(d time.Duration) Option {
	return func(ja *JwtAuth) {
		ja.signedTokenDuration = d
	}
}

func WithRefreshTokenDuration(d time.Duration) Option {
	return func(ja *JwtAuth) {
		ja.signedRefreshDuration = d
	}
}

func (g *JwtAuth) GenerateSignedToken(subject, username string, roles ...string) (*authorizer.Token, error) {
	expiresAt := time.Now().Local().Add(g.signedTokenDuration)
	claims := &SignedDetails{
		Username: username,
		Roles:    strings.Join(roles, ","),
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

func (g *JwtAuth) GenerateRefreshToken(uid string) (*authorizer.Token, error) {

	expiresAt := time.Now().Local().Add(g.signedRefreshDuration)
	refreshClaims := &SignedDetails{
		StandardClaims: jwt.StandardClaims{
			Subject:   uid,
			ExpiresAt: expiresAt.Unix(),
		},
	}

	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString(g.secret)
	if err != nil {
		return nil, err
	}

	return &authorizer.Token{
		Token:     refreshToken,
		ExpiresAt: expiresAt,
	}, nil
}

func (g *JwtAuth) ParseToken(signedToken string) (*authorizer.Claims, error) {
	claims := &SignedDetails{}
	_, err := jwt.ParseWithClaims(signedToken, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return g.secret, nil
	})
	if err != nil {
		return nil, err
	}
	return &authorizer.Claims{
		Subject:  claims.Subject,
		Username: claims.Username,
		Roles:    claims.Roles,
		Exp:      claims.ExpiresAt,
	}, nil
}

func (g *JwtAuth) ParseRoles(roles string) []string {
	return strings.Split(roles, ",")
}

func (g *JwtAuth) SetTokenDuration(duration time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.signedTokenDuration = duration
}

func (g *JwtAuth) SetRefreshTokenDuration(duration time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.signedRefreshDuration = duration
}
