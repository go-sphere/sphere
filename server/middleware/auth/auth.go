package auth

import (
	"errors"
	"strings"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/server/auth/authorizer"
)

const (
	// AuthorizationHeader is the standard HTTP header for authentication tokens.
	AuthorizationHeader = "Authorization"
	// AuthorizationPrefixBearer is the standard Bearer token prefix.
	AuthorizationPrefixBearer = "Bearer"
)

func parserToken[T authorizer.UID, C authorizer.Claims[T]](ctx httpx.Context, token string, transform func(text string) (string, error), parser authorizer.Parser[T, C]) error {
	if token == "" {
		return authorizer.TokenNotFoundError
	}
	if transform != nil {
		tranToken, err := transform(token)
		if err != nil {
			return err
		}
		if tranToken == "" {
			return authorizer.TokenNotFoundError
		}
		token = tranToken
	}
	claims, err := parser.ParseToken(ctx.Context(), token)
	if err != nil {
		return err
	}

	var data authorizer.Data[T]
	// The identity is mandatory: without it a zero UID would be stored as a valid
	// authenticated user and pass every downstream ownership check. Enforce it
	// here too rather than trusting every Claims implementation to do so.
	data.UID, err = claims.GetUID()
	if err != nil {
		return err
	}
	var zero T
	if data.UID == zero {
		return authorizer.MissingUIDError
	}
	// Subject and roles are optional. Implementations that cannot supply them leave
	// the zero value, which only ever narrows what the request is allowed to do.
	if subject, e := claims.GetSubject(); e == nil {
		data.Subject = subject
	}
	if roles, e := claims.GetRoles(); e == nil {
		data.Roles = roles
	}
	ctx.SetContext(authorizer.WithAuthData[T](ctx.Context(), data))
	return nil
}

type options struct {
	loader       func(ctx httpx.Context) (string, error)
	transform    func(text string) (string, error)
	abortOnError bool
}

func newOptions(opts ...Option) *options {
	defaults := &options{
		loader: func(ctx httpx.Context) (string, error) {
			return ctx.Header(AuthorizationHeader), nil
		},
		abortOnError: true,
	}
	for _, opt := range opts {
		opt(defaults)
	}
	return defaults
}

// Option configures token loading, rewriting, and abort-on-error behavior for NewAuthMiddleware.
type Option func(*options)

// WithLoader replaces the default loader that reads the Authorization header.
// A loader error rejects the request with 401 unless WithAbortOnError(false)
// is set; an empty token is treated as missing. The last loader option wins.
func WithLoader(f func(ctx httpx.Context) (string, error)) Option {
	return func(opts *options) {
		opts.loader = f
	}
}

// WithHeaderLoader loads the token from the named request header.
func WithHeaderLoader(header string) Option {
	return WithLoader(func(ctx httpx.Context) (string, error) {
		return ctx.Header(header), nil
	})
}

// WithCookieLoader loads the token from the named cookie. A missing cookie is an error.
func WithCookieLoader(cookieName string) Option {
	return WithLoader(func(ctx httpx.Context) (string, error) {
		cookie, err := ctx.Cookie(cookieName)
		if err != nil {
			return "", err
		}
		return cookie, nil
	})
}

// WithTransform rewrites the loaded token before ParseToken. A transform error
// rejects the request, and an empty result is treated as a missing token. It
// replaces any earlier WithTransform or WithPrefixTransform.
func WithTransform(f func(text string) (string, error)) Option {
	return func(opts *options) {
		opts.transform = f
	}
}

// WithPrefixTransform strips prefix+" " when the token starts with it.
// The match is case-insensitive: HTTP auth schemes ("Bearer") are defined as
// case-insensitive, and compliant clients may send "bearer <token>".
// A missing prefix is not an error; the raw string is parsed.
func WithPrefixTransform(prefix string) Option {
	prefix = strings.TrimSpace(prefix)
	if len(prefix) > 0 {
		prefix = prefix + " "
	}
	return WithTransform(func(text string) (string, error) {
		if len(prefix) > 0 && len(text) >= len(prefix) && strings.EqualFold(text[:len(prefix)], prefix) {
			text = strings.TrimSpace(text[len(prefix):])
		}
		return text, nil
	})
}

// WithAbortOnError controls whether authentication failures should abort the request.
// When set to false, authentication errors are ignored and the request continues.
// Requests with a valid token still get auth data; others continue without it, so
// handlers must check for it (authorizer.ContextUtils reports NeedLoginError).
// The default is true.
func WithAbortOnError(abort bool) Option {
	return func(opts *options) {
		opts.abortOnError = abort
	}
}

func unauthorizedError(err error) error {
	var messageErr httpx.MessageError
	if errors.As(err, &messageErr) {
		return httpx.UnauthorizedError(err, messageErr.GetMessage())
	}
	return httpx.UnauthorizedError(err)
}

// NewAuthMiddleware parses a request token with parser and stores authorizer.Data on the context.
//
// For each request it loads the token (Authorization header by default), applies
// the configured transform, calls parser.ParseToken with the request context,
// and stores UID, Subject, and Roles. The request is rejected with a 401 error
// (keeping the underlying error's user-facing message) when loading fails, the
// token is empty (authorizer.TokenNotFoundError), parsing fails, or GetUID
// fails or yields a zero UID (authorizer.MissingUIDError). GetSubject and
// GetRoles errors leave those fields empty instead.
//
// With WithAbortOnError(false), every failure is ignored and the request
// continues without auth data. The parser must be safe for concurrent use.
func NewAuthMiddleware[T authorizer.UID, C authorizer.Claims[T]](parser authorizer.Parser[T, C], options ...Option) httpx.Middleware {
	opts := newOptions(options...)
	return func(next httpx.Handler) httpx.Handler {
		return func(ctx httpx.Context) error {
			token, err := opts.loader(ctx)
			if err != nil && opts.abortOnError {
				return unauthorizedError(err)
			}
			err = parserToken(ctx, token, opts.transform, parser)
			if err != nil && opts.abortOnError {
				return unauthorizedError(err)
			}
			return next(ctx)
		}
	}
}
