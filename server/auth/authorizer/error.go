package authorizer

import (
	"errors"

	"github.com/go-sphere/httpx"
)

var (
	TokenNotFoundError = httpx.UnauthorizedError(
		errors.New("AuthorizerError:TOKEN_NOT_FOUND"),
		"没有提供有效的认证信息",
	)
	NeedLoginError = httpx.UnauthorizedError(
		errors.New("AuthorizerError:NEED_LOGIN"),
		"需要登录才能访问",
	)
	PermissionError = httpx.ForbiddenError(
		errors.New("AuthorizerError:PERMISSION_DENIED"),
		"没有权限访问该资源",
	)
)
