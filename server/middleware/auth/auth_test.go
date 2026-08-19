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
