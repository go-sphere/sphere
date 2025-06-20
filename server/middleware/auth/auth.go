package auth

import (
	"net/http"
	"strings"

	"github.com/TBXark/sphere/server/auth/authorizer"
	"github.com/gin-gonic/gin"
)

const (
	AuthorizationHeader = "Authorization"
)

func abortUnauthorized(ctx *gin.Context) {
	ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"error":   "unauthorized",
		"message": "没有提供有效的认证信息",
	})
}

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

func NewCookieAuthMiddleware[T authorizer.UID, C authorizer.Claims[T]](cookieName string, transform func(raw string) (string, error), parser authorizer.Parser[T, C], abortOnError bool) gin.HandlerFunc {
	return NewCommonAuthMiddleware[T](func(ctx *gin.Context) (string, error) {
		return ctx.Cookie(cookieName)
	}, transform, parser, abortOnError)
}

func NewAuthMiddleware[T authorizer.UID, C authorizer.Claims[T]](prefix string, parser authorizer.Parser[T, C], abortOnError bool) gin.HandlerFunc {
	prefix = strings.TrimSpace(prefix)
	if len(prefix) > 0 {
		prefix = prefix + " "
	}
	return NewCommonAuthMiddleware[T](
		func(ctx *gin.Context) (string, error) {
			return ctx.GetHeader(AuthorizationHeader), nil
		},
		func(token string) (string, error) {
			if len(prefix) > 0 && strings.HasPrefix(token, prefix) {
				token = strings.TrimSpace(strings.TrimPrefix(token, prefix))
			}
			return token, nil
		},
		parser,
		abortOnError,
	)
}

func NewCommonAuthMiddleware[T authorizer.UID, C authorizer.Claims[T]](loader func(ctx *gin.Context) (string, error), transform func(text string) (string, error), parser authorizer.Parser[T, C], abortOnError bool) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token, err := loader(ctx)
		if err != nil && abortOnError {
			abortUnauthorized(ctx)
			return
		}
		err = parserToken(ctx, token, transform, parser)
		if err != nil && abortOnError {
			abortUnauthorized(ctx)
			return
		}
		ctx.Next()
	}
}
