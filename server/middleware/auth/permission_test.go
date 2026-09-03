package auth

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/server/auth/acl"
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

// TestPermissionMiddleware_AdversarialStress tests permission middleware with ACL
// under 100+ concurrent requests.
func TestPermissionMiddleware_AdversarialStress(t *testing.T) {
	t.Parallel()

	accessControl := acl.NewACL()
	accessControl.Allow("admin", "/api/v1/admin")
	accessControl.Allow("admin", "/api/v1/metrics")
	accessControl.Allow("operator", "/api/v1/metrics")
	accessControl.Allow("user", "/api/v1/profile")

	adminMw := NewPermissionMiddleware[int64]("/api/v1/admin", accessControl)
	metricsMw := NewPermissionMiddleware[int64]("/api/v1/metrics", accessControl)

	const concurrency = 100
	var wg sync.WaitGroup
	wg.Add(concurrency * 3)

	// 1. Admin accessing /admin -> Allowed
	for i := range concurrency {
		go func(id int) {
			defer wg.Done()
			ctx := newStressFakeContext(nil, nil)
			ctx.SetContext(authorizer.WithAuthData[int64](context.Background(), authorizer.Data[int64]{
				UID:   int64(id),
				Roles: []string{"admin"},
			}))

			err := adminMw(ctx)
			if err != nil {
				t.Errorf("admin request %d failed: %v", id, err)
				return
			}
			if !ctx.nextInvoked.Load() {
				t.Errorf("admin request %d did not call Next()", id)
			}
		}(i)
	}

	// 2. User accessing /admin -> 403 Forbidden
	for i := range concurrency {
		go func(id int) {
			defer wg.Done()
			ctx := newStressFakeContext(nil, nil)
			ctx.SetContext(authorizer.WithAuthData[int64](context.Background(), authorizer.Data[int64]{
				UID:   int64(id),
				Roles: []string{"user"},
			}))

			err := adminMw(ctx)
			if err == nil {
				t.Errorf("user request %d to admin endpoint should fail", id)
				return
			}
			_, status, _ := httpx.ParseError(err)
			if status != http.StatusForbidden {
				t.Errorf("user request %d status = %d, want 403", id, status)
			}
			if ctx.nextInvoked.Load() {
				t.Errorf("user request %d must not call Next()", id)
			}
		}(i)
	}

	// 3. Unauthenticated request to /metrics -> 403 Forbidden
	for i := range concurrency {
		go func(id int) {
			defer wg.Done()
			ctx := newStressFakeContext(nil, nil)

			err := metricsMw(ctx)
			if err == nil {
				t.Errorf("unauth request %d to metrics endpoint should fail", id)
				return
			}
			_, status, _ := httpx.ParseError(err)
			if status != http.StatusForbidden {
				t.Errorf("unauth request %d status = %d, want 403", id, status)
			}
			if ctx.nextInvoked.Load() {
				t.Errorf("unauth request %d must not call Next()", id)
			}
		}(i)
	}

	wg.Wait()
}
