package jwtauth

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJwtAuth_GenerateToken(t *testing.T) {
	claims := NewRBACClaims[int64](12345, "test-subject", []string{"admin", "user"}, time.Now().Add(1*time.Hour))
	jwtAuth := NewJwtAuth[RBACClaims[int64]]("secret")
	token, err := jwtAuth.GenerateToken(context.Background(), claims)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	t.Log(token)
	parsedClaims, err := jwtAuth.ParseToken(context.Background(), token)
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}
	if parsedClaims.UID != claims.UID {
		t.Errorf("expected UID %d, got %d", claims.UID, parsedClaims.UID)
	}
	if len(parsedClaims.Roles) != len(claims.Roles) {
		t.Errorf("expected roles %v, got %v", claims.Roles, parsedClaims.Roles)
	}
	for i, role := range claims.Roles {
		if parsedClaims.Roles[i] != role {
			t.Errorf("expected role %s, got %s", role, parsedClaims.Roles[i])
		}
	}
	if parsedClaims.Subject != claims.Subject {
		t.Errorf("expected subject %s, got %s", claims.Subject, parsedClaims.Subject)
	}

	jwtAuth2 := NewJwtAuth[RBACClaims[int64]]("secret", WithSigningMethod(jwt.SigningMethodHS512))
	_, err = jwtAuth2.ParseToken(context.Background(), token)
	if err == nil {
		t.Error("expected error, got nil")
	}
	t.Log(err)
}
