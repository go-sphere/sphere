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
	// MissingUIDError is returned by Claims.GetUID when the token carries no
	// identity. A zero UID must never be treated as an authenticated user: it
	// would be stored as valid auth data and satisfy every "is logged in" check,
	// so any token signed with the same secret for another purpose (password
	// reset, email verification, an internal service token) would work as an
	// access token.
	MissingUIDError = httpx.UnauthorizedError(
		errors.New("AuthorizerError:MISSING_UID"),
		"认证信息缺少用户标识",
	)
)
