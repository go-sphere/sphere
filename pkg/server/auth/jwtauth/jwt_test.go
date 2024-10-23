package jwtauth

import (
	"github.com/tbxark/sphere/pkg/server/auth/authorizer"
	"testing"
	"time"
)

func TestJwtAuth_ParseToken(t *testing.T) {
	auth := NewJwtAuth[string]("secret")
	token, err := auth.GenerateToken(authorizer.NewClaims[string]("1", "username", "admin", time.Now().Add(time.Hour).Unix()))
	if err != nil {
		t.Error(err)
	}
	claims, err := auth.ParseToken(token.Token)
	if err != nil {
		t.Error(err)
	}
	if claims.Subject != "username" {
		t.Error("subject not match")
	}
	if claims.Roles != "admin" {
		t.Error("roles not match")
	}
	if claims.UID != "1" {
		t.Error("uid not match")
	}
	token, err = auth.GenerateToken(authorizer.NewClaims[string]("1", "username", "admin", time.Now().Add(-time.Hour).Unix()))
	if err != nil {
		t.Error(err)
	}
	time.Sleep(time.Second)
	_, err = auth.ParseToken(token.Token)
	if err == nil {
		t.Error("token should be expired")
	}
	t.Logf("error: %v", err)
}
