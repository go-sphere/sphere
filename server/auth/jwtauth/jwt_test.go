package jwtauth

import (
	"encoding/json"
	"github.com/TBXark/sphere/server/auth/authorizer"
	"github.com/golang-jwt/jwt/v5"
	"testing"
	"time"
)

func TestJwtAuth_ParseToken(t *testing.T) {
	auth := NewJwtAuth[authorizer.RBACClaims[int64]]("secret")
	info := authorizer.NewRBACClaims[int64](1, "username", []string{"admin"}, time.Now().Add(time.Hour))
	token, err := auth.GenerateToken(info)
	if err != nil {
		t.Error(err)
	}
	claims1, err := auth.ParseToken(token)
	if err != nil {
		t.Error(err)
	}
	if claims1.Subject != "username" {
		t.Error("subject not match")
	}
	if claims1.Roles[0] != "admin" {
		t.Error("roles not match")
	}
	if claims1.UID != 1 {
		t.Error("uid not match")
	}
	info = authorizer.NewRBACClaims[int64](1, "username", []string{"admin"}, time.Now().Add(-time.Hour))
	token, err = auth.GenerateToken(info)
	if err != nil {
		t.Error(err)
	}
	time.Sleep(time.Second)
	_, err = auth.ParseToken(token)
	if err == nil {
		t.Error("token should be expired")
	}
	t.Logf("error: %v", err)

}

func parseClaimsV1[T jwt.Claims](raw []byte) (*T, error) {
	var claims T
	err := json.Unmarshal(raw, &claims)
	if err != nil {
		return nil, err
	}
	return &claims, nil
}

func parseClaimsV2(claims jwt.Claims, raw []byte) (jwt.Claims, error) {
	err := json.Unmarshal(raw, &claims)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

func TestJwtAuth_JSON(t *testing.T) {
	//
	// This is a test for the ParseUnverified method in the jwt package
	// func (p *Parser) ParseUnverified(tokenString string, claims Claims) (token *Token, parts []string, err error) {
	//

	// jwt.Claims is an interface. The claims value you pass into the parser needs to be a concrete type. It's essentially passed directly through to the standard library JSON parser and will follow that behavior.

	info := authorizer.NewRBACClaims[int64](1, "username", []string{"admin"}, time.Now().Add(time.Hour))
	raw, err := json.Marshal(info)
	if err != nil {
		t.Error(err)
	}
	claims, err := parseClaimsV1[authorizer.RBACClaims[int64]](raw)
	if err != nil {
		t.Error(err)
	}
	if claims.Subject != info.Subject {
		t.Error("subject not match")
	}

	var claimsV2 authorizer.RBACClaims[int64]
	_, err = parseClaimsV2(claimsV2, raw)
	if err != nil {
		t.Log(err)
	}

	data, err := parseClaimsV2(&claimsV2, raw)
	if err != nil {
		t.Error(err)
	}
	t.Logf("data: %v", data)
}
