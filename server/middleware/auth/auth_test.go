package auth

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/server/auth/authorizer"
)

// httpxContext aliases httpx.Context so the embedded field name does not collide
// with the interface's own Context() method (which would shadow it).
type httpxContext = httpx.Context

// fakeContext embeds httpx.Context so it satisfies the full interface while only
// overriding the two methods parserToken depends on.
type fakeContext struct {
	httpxContext
	ctx context.Context
}

func (f *fakeContext) Context() context.Context {
	if f.ctx == nil {
		return context.Background()
	}
	return f.ctx
}

func (f *fakeContext) SetContext(ctx context.Context) {
	f.ctx = ctx
}

type stubClaims struct {
	uid        int64
	uidErr     error
	subject    string
	subjectErr error
	roles      []string
	rolesErr   error
}

func (c stubClaims) GetUID() (int64, error)      { return c.uid, c.uidErr }
func (c stubClaims) GetSubject() (string, error) { return c.subject, c.subjectErr }
func (c stubClaims) GetRoles() ([]string, error) { return c.roles, c.rolesErr }

func TestParserTokenClaimsErrors(t *testing.T) {
	t.Parallel()

	uidErr := errors.New("uid unavailable")

	tests := []struct {
		name        string
		claims      stubClaims
		wantErr     error
		wantData    bool
		wantUID     int64
		wantSubject string
		wantRoles   []string
	}{
		{
			name:    "uid error rejects the request",
			claims:  stubClaims{uidErr: uidErr, subject: "alice", roles: []string{"admin"}},
			wantErr: uidErr,
		},
		{
			// A claims implementation that returns (zero, nil) instead of
			// MissingUIDError must not authenticate: every token signed with
			// the same secret would otherwise share one identity.
			name:    "zero uid without error is rejected",
			claims:  stubClaims{uid: 0, subject: "alice", roles: []string{"admin"}},
			wantErr: authorizer.MissingUIDError,
		},
		{
			name:      "subject error leaves subject empty",
			claims:    stubClaims{uid: 7, subjectErr: errors.New("no subject"), roles: []string{"admin"}},
			wantData:  true,
			wantUID:   7,
			wantRoles: []string{"admin"},
		},
		{
			name:        "roles error leaves roles nil",
			claims:      stubClaims{uid: 7, subject: "alice", rolesErr: errors.New("no roles")},
			wantData:    true,
			wantUID:     7,
			wantSubject: "alice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := &fakeContext{}
			parser := authorizer.ParserFunc[int64, stubClaims](func(context.Context, string) (stubClaims, error) {
				return tt.claims, nil
			})

			err := parserToken[int64, stubClaims](ctx, "token", nil, parser)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("parserToken() error = %v, want %v", err, tt.wantErr)
			}

			data, ok := authorizer.GetAuthData[int64](ctx.Context())
			if ok != tt.wantData {
				t.Fatalf("auth data present = %v, want %v", ok, tt.wantData)
			}
			if !tt.wantData {
				return
			}
			if data.UID != tt.wantUID {
				t.Errorf("UID = %d, want %d", data.UID, tt.wantUID)
			}
			if data.Subject != tt.wantSubject {
				t.Errorf("Subject = %q, want %q", data.Subject, tt.wantSubject)
			}
			if !slices.Equal(data.Roles, tt.wantRoles) {
				t.Errorf("Roles = %v, want %v", data.Roles, tt.wantRoles)
			}
		})
	}
}

func TestUnauthorizedErrorPreservesUserMessage(t *testing.T) {
	t.Parallel()

	err := unauthorizedError(authorizer.TokenNotFoundError)
	code, status, message := httpx.ParseError(err)
	wantCode, wantStatus, wantMessage := httpx.ParseError(authorizer.TokenNotFoundError)

	if code != wantCode || status != wantStatus || message != wantMessage {
		t.Fatalf(
			"unauthorized error mismatch: got=(%d, %d, %q) want=(%d, %d, %q)",
			code,
			status,
			message,
			wantCode,
			wantStatus,
			wantMessage,
		)
	}
}

type fullFakeContext struct {
	httpxContext
	ctx     context.Context
	headers map[string]string
	cookies map[string]string
	nexted  bool
}

func (f *fullFakeContext) Context() context.Context {
	if f.ctx == nil {
		return context.Background()
	}
	return f.ctx
}

func (f *fullFakeContext) SetContext(ctx context.Context) {
	f.ctx = ctx
}

func (f *fullFakeContext) Header(key string) string {
	if f.headers == nil {
		return ""
	}
	return f.headers[key]
}

func (f *fullFakeContext) Cookie(name string) (string, error) {
	if f.cookies == nil {
		return "", errors.New("no cookies")
	}
	val, ok := f.cookies[name]
	if !ok {
		return "", errors.New("cookie not found")
	}
	return val, nil
}

// run drives mw with a terminal handler that records whether the chain
// continued past the middleware.
func run(mw httpx.Middleware, ctx *fullFakeContext) error {
	return mw(func(httpx.Context) error {
		ctx.nexted = true
		return nil
	})(ctx)
}

func TestWithPrefixTransform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prefix string
		text   string
		want   string
	}{
		{name: "standard prefix", prefix: "Bearer", text: "Bearer my-token-123", want: "my-token-123"},
		{name: "prefix requires whitespace boundary", prefix: "Bearer", text: "BearerToken", want: "BearerToken"},
		{name: "multiple spaces are trimmed", prefix: "Bearer", text: "Bearer   my-token-123  ", want: "my-token-123"},
		{name: "prefix option is trimmed", prefix: " Bearer ", text: "Bearer my-token-123", want: "my-token-123"},
		{name: "missing prefix preserves input", prefix: "Bearer", text: "my-token-123", want: "my-token-123"},
		{name: "empty prefix preserves input", prefix: "", text: "  my-token-123  ", want: "  my-token-123  "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opt := newOptions(WithPrefixTransform(tt.prefix))
			got, err := opt.transform(tt.text)
			if err != nil {
				t.Fatalf("transform error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("transform got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewAuthMiddleware(t *testing.T) {
	t.Parallel()

	parser := authorizer.ParserFunc[int64, stubClaims](func(ctx context.Context, token string) (stubClaims, error) {
		if token == "valid-token" {
			return stubClaims{uid: 100, subject: "alice", roles: []string{"user"}}, nil
		}
		return stubClaims{}, errors.New("invalid token")
	})

	t.Run("header loader success", func(t *testing.T) {
		mw := NewAuthMiddleware(parser,
			WithHeaderLoader("X-Custom-Auth"),
			WithPrefixTransform("Token"),
		)
		ctx := &fullFakeContext{
			headers: map[string]string{"X-Custom-Auth": "Token valid-token"},
		}
		if err := run(mw, ctx); err != nil {
			t.Fatalf("mw error: %v", err)
		}
		if !ctx.nexted {
			t.Fatal("the chain did not continue")
		}
		data, ok := authorizer.GetAuthData[int64](ctx.Context())
		if !ok || data.UID != 100 {
			t.Fatalf("auth data UID = %d, want 100", data.UID)
		}
	})

	t.Run("cookie loader success", func(t *testing.T) {
		mw := NewAuthMiddleware(parser, WithCookieLoader("session_id"))
		ctx := &fullFakeContext{
			cookies: map[string]string{"session_id": "valid-token"},
		}
		if err := run(mw, ctx); err != nil {
			t.Fatalf("mw error: %v", err)
		}
		if !ctx.nexted {
			t.Fatal("the chain did not continue")
		}
	})

	t.Run("invalid token with abort on error", func(t *testing.T) {
		mw := NewAuthMiddleware(parser, WithAbortOnError(true))
		ctx := &fullFakeContext{
			headers: map[string]string{AuthorizationHeader: "invalid-token"},
		}
		err := run(mw, ctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		_, status, _ := httpx.ParseError(err)
		if status != 401 {
			t.Fatalf("status = %d, want 401", status)
		}
		if ctx.nexted {
			t.Fatal("the chain continued although the request was aborted")
		}
	})

	t.Run("invalid token without abort on error", func(t *testing.T) {
		mw := NewAuthMiddleware(parser, WithAbortOnError(false))
		ctx := &fullFakeContext{
			headers: map[string]string{AuthorizationHeader: "invalid-token"},
		}
		if err := run(mw, ctx); err != nil {
			t.Fatalf("expected nil error when abortOnError is false, got: %v", err)
		}
		if !ctx.nexted {
			t.Fatal("the chain did not continue with abortOnError false")
		}
	})
}
