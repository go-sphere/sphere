package docs

import (
	_ "github.com/tbxark/go-base-api/internal/biz/api"
	_ "github.com/tbxark/go-base-api/internal/biz/dash"
)

// @securityDefinitions.apikey  ApiKeyAuth
// @in                          header
// @name                        Authorization
// @description                 JWT token
