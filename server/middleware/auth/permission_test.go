package auth

import (
	"context"
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/httpxmock"
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
		ctx := httpxmock.New(nil)
		ctx.SetContext(authorizer.WithAuthData[int64](context.Background(), authorizer.Data[int64]{
			UID:   1,
			Roles: []string{"user", "admin"},
		}))
		next := &httpxmock.Handler{}
		if err := httpxmock.Run(ctx, next.Handle, mw); err != nil {
			t.Fatalf("mw error: %v", err)
		}
		if !next.Called() {
			t.Fatal("the chain did not continue for an authorized user")
		}
	})

	t.Run("disallowed role returns forbidden", func(t *testing.T) {
		ctx := httpxmock.New(nil)
		ctx.SetContext(authorizer.WithAuthData[int64](context.Background(), authorizer.Data[int64]{
			UID:   2,
			Roles: []string{"user"},
		}))
		next := &httpxmock.Handler{}
		err := httpxmock.Run(ctx, next.Handle, mw)
		if err == nil {
			t.Fatal("expected permission error, got nil")
		}
		_, status, _ := httpx.ParseError(err)
		if status != 403 {
			t.Fatalf("status = %d, want 403", status)
		}
		if next.Called() {
			t.Fatal("the chain continued although access was denied")
		}
	})

	t.Run("no auth data in context returns forbidden", func(t *testing.T) {
		ctx := httpxmock.New(nil)
		next := &httpxmock.Handler{}
		err := httpxmock.Run(ctx, next.Handle, mw)
		if err == nil {
			t.Fatal("expected permission error, got nil")
		}
		_, status, _ := httpx.ParseError(err)
		if status != 403 {
			t.Fatalf("status = %d, want 403", status)
		}
	})
}
