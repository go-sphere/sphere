package jwtauth

import (
	"testing"
	"time"
)

func TestJwtAuth_ParseToken(t *testing.T) {
	auth := NewJwtAuth("secret")
	token, err := auth.GenerateSignedToken("1", "username", "admin")
	if err != nil {
		t.Error(err)
	}
	claims, err := auth.ParseToken(token.Token)
	if err != nil {
		t.Error(err)
	}
	if claims.Username != "username" {
		t.Error("username not match")
	}
	if claims.Roles != "admin" {
		t.Error("roles not match")
	}
	if claims.Subject != "1" {
		t.Error("subject not match")
	}
	auth.SetTokenDuration(0)
	token, err = auth.GenerateSignedToken("1", "username", "admin")
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
