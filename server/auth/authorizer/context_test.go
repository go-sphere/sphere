package authorizer

import (
	"context"
	"errors"
	"testing"

	"github.com/go-sphere/httpx"
)

type int64Utils = ContextUtils[int64]
type stringUtils = ContextUtils[string]

func requireStatus(tb testing.TB, err error, want int) {
	tb.Helper()
	if err == nil {
		tb.Fatal("got nil error, want a status-carrying one")
	}
	var se httpx.StatusError
	if !errors.As(err, &se) {
		tb.Fatalf("%T does not carry an HTTP status", err)
	}
	if got := int(se.GetStatus()); got != want {
		tb.Errorf("status = %d, want %d", got, want)
	}
}

// TestWithAndGetAuthData pins the round trip that every later helper depends on,
// and the type-mismatch isolation that keeps a context keyed for one UID type
// from being read as another.
func TestWithAndGetAuthData(t *testing.T) {
	t.Run("round trip", func(t *testing.T) {
		ctx := WithAuthData[int64](context.Background(), Data[int64]{UID: 42, Subject: "alice", Roles: []string{"admin"}})

		got, ok := GetAuthData[int64](ctx)
		if !ok {
			t.Fatal("stored auth data was not found")
		}
		if got.UID != 42 || got.Subject != "alice" || len(got.Roles) != 1 || got.Roles[0] != "admin" {
			t.Fatalf("got %+v, want the stored data", got)
		}
	})

	t.Run("absent", func(t *testing.T) {
		if _, ok := GetAuthData[int64](context.Background()); ok {
			t.Fatal("an empty context must not yield auth data")
		}
	})

	t.Run("type mismatch", func(t *testing.T) {
		// A context populated for int64 UIDs must not satisfy string reads. Both
		// are zero-length-error-free paths only if the type assertion fails closed.
		ctx := WithAuthData[int64](context.Background(), Data[int64]{UID: 7})
		if got, ok := GetAuthData[string](ctx); ok {
			t.Fatalf("cross-type read yielded %+v, want not found", got)
		}
	})

	t.Run("wrong kind stored", func(t *testing.T) {
		// A different value under the same key (a hand-rolled context) must be
		// rejected, not interpreted.
		ctx := context.WithValue(context.Background(), authContextKey, Data[string]{UID: "u1"})
		if _, ok := GetAuthData[int64](ctx); ok {
			t.Fatal("a mismatched stored type must not satisfy the read")
		}
	})
}

// TestGetCurrentID pins the primary identity helper: it must fail with the login
// sentinel when unauthenticated, and never conflate that with a zero UID.
func TestGetCurrentID(t *testing.T) {
	t.Run("unauthenticated", func(t *testing.T) {
		var cu int64Utils
		got, err := cu.GetCurrentID(context.Background())
		if !errors.Is(err, NeedLoginError) {
			t.Fatalf("err = %v, want NeedLoginError", err)
		}
		if got != 0 {
			t.Fatalf("zero UID = %d, want the type's zero value", got)
		}
		requireStatus(t, err, 401)
	})

	t.Run("authenticated", func(t *testing.T) {
		var cu int64Utils
		ctx := WithAuthData[int64](context.Background(), Data[int64]{UID: 42})
		got, err := cu.GetCurrentID(ctx)
		if err != nil {
			t.Fatalf("GetCurrentID: %v", err)
		}
		if got != 42 {
			t.Fatalf("UID = %d, want 42", got)
		}
	})

	t.Run("string UID", func(t *testing.T) {
		var cu stringUtils
		ctx := WithAuthData[string](context.Background(), Data[string]{UID: "u-9"})
		got, err := cu.GetCurrentID(ctx)
		if err != nil {
			t.Fatalf("GetCurrentID: %v", err)
		}
		if got != "u-9" {
			t.Fatalf("UID = %q, want u-9", got)
		}
	})
}

// TestCheckAuthStatus pins the presence-only check.
func TestCheckAuthStatus(t *testing.T) {
	var cu int64Utils
	if err := cu.CheckAuthStatus(context.Background()); !errors.Is(err, NeedLoginError) {
		t.Fatalf("unauthenticated: err = %v, want NeedLoginError", err)
	}

	ctx := WithAuthData[int64](context.Background(), Data[int64]{UID: 1})
	if err := cu.CheckAuthStatus(ctx); err != nil {
		t.Fatalf("authenticated: err = %v, want nil", err)
	}
}

// TestCheckAuthID pins the ownership gate: it must distinguish "not logged in"
// from "logged in as someone else", and treat a matching ID as allowed.
func TestCheckAuthID(t *testing.T) {
	var cu int64Utils

	t.Run("unauthenticated", func(t *testing.T) {
		if err := cu.CheckAuthID(context.Background(), 0); !errors.Is(err, NeedLoginError) {
			t.Fatalf("err = %v, want NeedLoginError", err)
		}
	})

	t.Run("mismatch", func(t *testing.T) {
		ctx := WithAuthData[int64](context.Background(), Data[int64]{UID: 42})
		err := cu.CheckAuthID(ctx, 7)
		if !errors.Is(err, PermissionError) {
			t.Fatalf("err = %v, want PermissionError", err)
		}
		requireStatus(t, err, 403)
	})

	t.Run("match", func(t *testing.T) {
		ctx := WithAuthData[int64](context.Background(), Data[int64]{UID: 42})
		if err := cu.CheckAuthID(ctx, 42); err != nil {
			t.Fatalf("err = %v, want nil for the owner", err)
		}
	})
}

// TestGetCurrentSubject pins the subject helper and its login sentinel.
func TestGetCurrentSubject(t *testing.T) {
	var cu int64Utils

	if _, err := cu.GetCurrentSubject(context.Background()); !errors.Is(err, NeedLoginError) {
		t.Fatalf("unauthenticated: err = %v, want NeedLoginError", err)
	}

	ctx := WithAuthData[int64](context.Background(), Data[int64]{UID: 42, Subject: "alice"})
	got, err := cu.GetCurrentSubject(ctx)
	if err != nil {
		t.Fatalf("GetCurrentSubject: %v", err)
	}
	if got != "alice" {
		t.Fatalf("subject = %q, want alice", got)
	}
}

// TestGetCurrentRoles pins the roles helper, including the documented nil for an
// unauthenticated caller — callers ranging over the result must be safe.
func TestGetCurrentRoles(t *testing.T) {
	var cu int64Utils

	if got := cu.GetCurrentRoles(context.Background()); got != nil {
		t.Fatalf("unauthenticated roles = %v, want nil", got)
	}

	ctx := WithAuthData[int64](context.Background(), Data[int64]{UID: 42, Roles: []string{"admin", "billing"}})
	got := cu.GetCurrentRoles(ctx)
	if len(got) != 2 || got[0] != "admin" || got[1] != "billing" {
		t.Fatalf("roles = %v, want [admin billing]", got)
	}

	// No roles set is still authenticated.
	ctx = WithAuthData[int64](context.Background(), Data[int64]{UID: 42})
	if got := cu.GetCurrentRoles(ctx); got != nil {
		t.Fatalf("roles without data = %v, want nil", got)
	}
}

// TestSentinelErrorsCarryStatuses pins that the exported sentinels carry the
// status codes callers rely on when they return these errors directly from
// handlers, so a wiring mistake in httpx.NewError shows up here rather than as a
// wrong status on the wire.
func TestSentinelErrorsCarryStatuses(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{name: "TokenNotFoundError", err: TokenNotFoundError, want: 401},
		{name: "NeedLoginError", err: NeedLoginError, want: 401},
		{name: "PermissionError", err: PermissionError, want: 403},
		{name: "MissingUIDError", err: MissingUIDError, want: 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireStatus(t, tc.err, tc.want)
		})
	}
}
