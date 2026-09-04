package auth

import (
	"context"
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/server/auth/authorizer"
)

type fakeACL struct {
	allowed map[string]map[string]bool
}

func (f *fakeACL) IsAllowed(role, resource string) bool {
	if m, ok := f.allowed[role]; ok {
		return m[resource]
	}
	return false
}

func TestNewPermissionMiddleware(t *testing.T) {
	t.Parallel()

	acl := &fakeACL{
		allowed: map[string]map[string]bool{
			"admin": {"/admin": true, "/dashboard": true},
			"user":  {"/dashboard": true},
		},
	}

	mw := NewPermissionMiddleware[int64]("/admin", acl)

	t.Run("allowed role succeeds", func(t *testing.T) {
		ctx := &fullFakeContext{}
		ctx.SetContext(authorizer.WithAuthData[int64](context.Background(), authorizer.Data[int64]{
			UID:   1,
			Roles: []string{"user", "admin"},
		}))
		if err := mw(ctx); err != nil {
			t.Fatalf("mw error: %v", err)
		}
		if !ctx.nexted {
			t.Fatal("Next() must be called for authorized user")
		}
	})

	t.Run("disallowed role returns forbidden", func(t *testing.T) {
		ctx := &fullFakeContext{}
		ctx.SetContext(authorizer.WithAuthData[int64](context.Background(), authorizer.Data[int64]{
			UID:   2,
			Roles: []string{"user"},
		}))
		err := mw(ctx)
		if err == nil {
			t.Fatal("expected permission error, got nil")
		}
		_, status, _ := httpx.ParseError(err)
		if status != 403 {
			t.Fatalf("status = %d, want 403", status)
		}
		if ctx.nexted {
			t.Fatal("Next() must not be called when denied")
		}
	})

	t.Run("no auth data in context returns forbidden", func(t *testing.T) {
		ctx := &fullFakeContext{}
		err := mw(ctx)
		if err == nil {
			t.Fatal("expected permission error, got nil")
		}
		_, status, _ := httpx.ParseError(err)
		if status != 403 {
			t.Fatalf("status = %d, want 403", status)
		}
	})
}
