package jwt_auth

import (
	"fmt"
	"github.com/golang-jwt/jwt"
	"strings"
	"time"
)

const (
	SignedDetailsUidKey       = "uid"
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
		Roles:    g.genRolesString(roles),
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

func (g *JwtAuth) Validate(signedToken string) (map[string]any, error) {
	token, err := jwt.ParseWithClaims(signedToken, &SignedDetails{}, func(token *jwt.Token) (interface{}, error) {
		return g.secretKey, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*SignedDetails)
	if !ok {
		return nil, fmt.Errorf("token is invalid")
	}

	var res = make(map[string]any)
	res["uid"] = claims.UID
	res["username"] = claims.Username
	res["roles"] = claims.Roles
	res["exp"] = claims.ExpiresAt

	return res, nil
}

func (g *JwtAuth) genRolesString(roles []string) string {
	return strings.Join(roles, ",")
}

func (g *JwtAuth) ParseRolesString(roles string) map[string]struct{} {
	roleMap := make(map[string]struct{})
	for _, r := range strings.Split(roles, ",") {
		roleMap[r] = struct{}{}
	}
	return roleMap
}
