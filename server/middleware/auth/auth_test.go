package auth

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/server/auth/authorizer"
	"github.com/go-sphere/sphere/server/auth/jwtauth"
	"github.com/golang-jwt/jwt/v5"
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

func (f *fullFakeContext) Next() error {
	f.nexted = true
	return nil
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
		if err := mw(ctx); err != nil {
			t.Fatalf("mw error: %v", err)
		}
		if !ctx.nexted {
			t.Fatal("Next() was not called")
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
		if err := mw(ctx); err != nil {
			t.Fatalf("mw error: %v", err)
		}
		if !ctx.nexted {
			t.Fatal("Next() was not called")
		}
	})

	t.Run("invalid token with abort on error", func(t *testing.T) {
		mw := NewAuthMiddleware(parser, WithAbortOnError(true))
		ctx := &fullFakeContext{
			headers: map[string]string{AuthorizationHeader: "invalid-token"},
		}
		err := mw(ctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		_, status, _ := httpx.ParseError(err)
		if status != 401 {
			t.Fatalf("status = %d, want 401", status)
		}
		if ctx.nexted {
			t.Fatal("Next() must not be called when aborted")
		}
	})

	t.Run("invalid token without abort on error", func(t *testing.T) {
		mw := NewAuthMiddleware(parser, WithAbortOnError(false))
		ctx := &fullFakeContext{
			headers: map[string]string{AuthorizationHeader: "invalid-token"},
		}
		if err := mw(ctx); err != nil {
			t.Fatalf("expected nil error when abortOnError is false, got: %v", err)
		}
		if !ctx.nexted {
			t.Fatal("Next() should be called when abortOnError is false")
		}
	})
}

type stressFakeContext struct {
	httpxContext
	ctx         context.Context
	headers     map[string]string
	cookies     map[string]string
	nextInvoked atomic.Bool
}

func newStressFakeContext(headers map[string]string, cookies map[string]string) *stressFakeContext {
	if headers == nil {
		headers = make(map[string]string)
	}
	if cookies == nil {
		cookies = make(map[string]string)
	}
	return &stressFakeContext{
		ctx:     context.Background(),
		headers: headers,
		cookies: cookies,
	}
}

func (s *stressFakeContext) Context() context.Context {
	if s.ctx == nil {
		return context.Background()
	}
	return s.ctx
}

func (s *stressFakeContext) SetContext(ctx context.Context) {
	s.ctx = ctx
}

func (s *stressFakeContext) Header(key string) string {
	return s.headers[key]
}

func (s *stressFakeContext) Cookie(name string) (string, error) {
	val, ok := s.cookies[name]
	if !ok {
		return "", errors.New("cookie not found")
	}
	return val, nil
}

func (s *stressFakeContext) Next() error {
	s.nextInvoked.Store(true)
	return nil
}

// TestAuthMiddleware_AdversarialStress tests auth middleware under 100+ concurrent requests
// with valid, invalid, expired, malformed, and missing tokens.
func TestAuthMiddleware_AdversarialStress(t *testing.T) {
	t.Parallel()

	secret := "auth-middleware-secret-key-12345"
	jwtHandler := jwtauth.NewJwtAuth[jwtauth.RBACClaims[int64]](secret)

	// Pre-generate tokens
	validToken, err := jwtHandler.GenerateToken(context.Background(), jwtauth.NewRBACClaims[int64](100, "alice", []string{"user", "admin"}, time.Now().Add(time.Hour)))
	if err != nil {
		t.Fatalf("Generate valid token: %v", err)
	}

	expiredClaims := jwtauth.RBACClaims[int64]{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "expired",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-10 * time.Minute)),
		},
		UID: 101,
	}
	expiredToken, err := jwtHandler.GenerateToken(context.Background(), expiredClaims)
	if err != nil {
		t.Fatalf("Generate expired token: %v", err)
	}

	zeroUIDClaims := jwtauth.NewRBACClaims[int64](0, "zero-user", []string{"user"}, time.Now().Add(time.Hour))
	zeroUIDToken, err := jwtHandler.GenerateToken(context.Background(), zeroUIDClaims)
	if err != nil {
		t.Fatalf("Generate zero UID token: %v", err)
	}

	authMw := NewAuthMiddleware[int64, jwtauth.RBACClaims[int64]](
		jwtHandler,
		WithPrefixTransform(AuthorizationPrefixBearer),
	)

	const concurrency = 100

	var wg sync.WaitGroup
	wg.Add(concurrency * 4)

	// 1. Concurrent valid requests
	for i := range concurrency {
		go func(id int) {
			defer wg.Done()
			ctx := newStressFakeContext(map[string]string{
				AuthorizationHeader: "Bearer " + validToken,
			}, nil)

			err := authMw(ctx)
			if err != nil {
				t.Errorf("valid request %d failed: %v", id, err)
				return
			}
			if !ctx.nextInvoked.Load() {
				t.Errorf("valid request %d did not invoke Next()", id)
				return
			}

			data, ok := authorizer.GetAuthData[int64](ctx.Context())
			if !ok || data.UID != 100 {
				t.Errorf("valid request %d expected UID 100, got %d (ok=%v)", id, data.UID, ok)
			}
		}(i)
	}

	// 2. Concurrent expired token requests (Must return 401, Next not called)
	for i := range concurrency {
		go func(id int) {
			defer wg.Done()
			ctx := newStressFakeContext(map[string]string{
				AuthorizationHeader: "Bearer " + expiredToken,
			}, nil)

			err := authMw(ctx)
			if err == nil {
				t.Errorf("expired token request %d should have failed", id)
				return
			}
			_, status, _ := httpx.ParseError(err)
			if status != http.StatusUnauthorized {
				t.Errorf("expired token request %d status = %d, want 401", id, status)
			}
			if ctx.nextInvoked.Load() {
				t.Errorf("expired token request %d must not invoke Next()", id)
			}
		}(i)
	}

	// 3. Concurrent zero UID token requests (Must return 401, Next not called)
	for i := range concurrency {
		go func(id int) {
			defer wg.Done()
			ctx := newStressFakeContext(map[string]string{
				AuthorizationHeader: "Bearer " + zeroUIDToken,
			}, nil)

			err := authMw(ctx)
			if err == nil {
				t.Errorf("zero UID request %d should have failed", id)
				return
			}
			_, status, _ := httpx.ParseError(err)
			if status != http.StatusUnauthorized {
				t.Errorf("zero UID request %d status = %d, want 401", id, status)
			}
			if ctx.nextInvoked.Load() {
				t.Errorf("zero UID request %d must not invoke Next()", id)
			}
		}(i)
	}

	// 4. Concurrent malformed / missing token requests (Must return 401, Next not called)
	for i := range concurrency {
		go func(id int) {
			defer wg.Done()
			var headerVal string
			switch id % 3 {
			case 0:
				headerVal = "" // missing
			case 1:
				headerVal = "Bearer " // empty after strip
			default:
				headerVal = "Bearer corrupted.jwt.token"
			}

			ctx := newStressFakeContext(map[string]string{
				AuthorizationHeader: headerVal,
			}, nil)

			err := authMw(ctx)
			if err == nil {
				t.Errorf("malformed request %d should have failed", id)
				return
			}
			_, status, _ := httpx.ParseError(err)
			if status != http.StatusUnauthorized {
				t.Errorf("malformed request %d status = %d, want 401", id, status)
			}
			if ctx.nextInvoked.Load() {
				t.Errorf("malformed request %d must not invoke Next()", id)
			}
		}(i)
	}

	wg.Wait()
}
