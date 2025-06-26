package authorizer

import (
	"errors"

	"github.com/TBXark/sphere/core/errors/statuserr"
)

var (
	TokenNotFoundError = statuserr.UnauthorizedError(
		errors.New("AuthorizerError:TOKEN_NOT_FOUND"),
		"没有提供有效的认证信息",
	)
	NeedLoginError = statuserr.UnauthorizedError(
		errors.New("AuthorizerError:NEED_LOGIN"),
		"需要登录才能访问",
	)
	PermissionError = statuserr.ForbiddenError(
		errors.New("AuthorizerError:PERMISSION_DENIED"),
		"没有权限访问该资源",
	)
)
