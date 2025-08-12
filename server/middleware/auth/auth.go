package auth

import (
	"net/http"
	"strings"

	"github.com/TBXark/sphere/server/auth/authorizer"
	"github.com/gin-gonic/gin"
)

const (
	AuthorizationHeader       = "Authorization"
	AuthorizationPrefixBearer = "Bearer"
)

func parserToken[T authorizer.UID, C authorizer.Claims[T]](ctx *gin.Context, token string, transform func(text string) (string, error), parser authorizer.Parser[T, C]) error {
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
	claims, err := parser.ParseToken(ctx, token)
	if err != nil {
		return err
	}

	if uid, e := claims.GetUID(); e == nil {
		ctx.Set(authorizer.ContextKeyUID, uid)
	}
	if subject, e := claims.GetSubject(); e == nil {
		ctx.Set(authorizer.ContextKeySubject, subject)
	}
	if roles, e := claims.GetRoles(); e == nil {
		ctx.Set(authorizer.ContextKeyRoles, roles)
	}
	return nil
}

type options struct {
	abortWithError func(ctx *gin.Context, err error)
	loader         func(ctx *gin.Context) (string, error)
	transform      func(text string) (string, error)
	abortOnError   bool
}

func newOptions(opts ...Option) *options {
	defaults := &options{
		abortWithError: func(ctx *gin.Context, err error) {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   err.Error(),
				"message": "没有提供有效的认证信息",
			})
		},
		loader: func(ctx *gin.Context) (string, error) {
			return ctx.GetHeader(AuthorizationHeader), nil
		},
		transform: func(text string) (string, error) {
			return text, nil
		},
		abortOnError: true,
	}
	for _, opt := range opts {
		opt(defaults)
	}
	return defaults
}

type Option func(*options)

func WithAbortWithError(f func(ctx *gin.Context, err error)) Option {
	return func(opts *options) {
		opts.abortWithError = f
	}
}

func WithLoader(f func(ctx *gin.Context) (string, error)) Option {
	return func(opts *options) {
		opts.loader = f
	}
}

func WithHeaderLoader(header string) Option {
	return WithLoader(func(ctx *gin.Context) (string, error) {
		return ctx.GetHeader(header), nil
	})
}

func WithCookieLoader(cookieName string) Option {
	return WithLoader(func(ctx *gin.Context) (string, error) {
		cookie, err := ctx.Cookie(cookieName)
		if err != nil {
			return "", err
		}
		return cookie, nil
	})
}

func WithTransform(f func(text string) (string, error)) Option {
	return func(opts *options) {
		opts.transform = f
	}
}

func WithPrefixTransform(prefix string) Option {
	prefix = strings.TrimSpace(prefix)
	if len(prefix) > 0 {
		prefix = prefix + " "
	}
	return WithTransform(func(text string) (string, error) {
		if len(prefix) > 0 && strings.HasPrefix(text, prefix) {
			text = strings.TrimSpace(strings.TrimPrefix(text, prefix))
		}
		return text, nil
	})
}

func WithAbortOnError(abort bool) Option {
	return func(opts *options) {
		opts.abortOnError = abort
	}
}

func NewAuthMiddleware[T authorizer.UID, C authorizer.Claims[T]](parser authorizer.Parser[T, C], options ...Option) gin.HandlerFunc {
	opts := newOptions(options...)
	return func(ctx *gin.Context) {
		token, err := opts.loader(ctx)
		if err != nil && opts.abortOnError {
			opts.abortWithError(ctx, err)
			return
		}
		err = parserToken(ctx, token, opts.transform, parser)
		if err != nil && opts.abortOnError {
			opts.abortWithError(ctx, err)
			return
		}
		ctx.Next()
	}
}
