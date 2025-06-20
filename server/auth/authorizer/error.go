package authorizer

import (
	"errors"
	"github.com/TBXark/sphere/server/statuserr"
)

var (
	TokenNotFoundError = statuserr.UnauthorizedError(errors.New("token not found"), "没有提供有效的认证信息")
	NeedLoginError     = statuserr.UnauthorizedError(errors.New("need login"), "需要登录才能访问")
	PermissionError    = statuserr.ForbiddenError(errors.New("no permission"), "没有权限访问该资源")
)
