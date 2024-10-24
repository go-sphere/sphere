package jwtauth

import (
	"github.com/tbxark/sphere/pkg/server/auth/authorizer"
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
