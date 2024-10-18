package sphere

import (
	_ "github.com/tbxark/sphere/api/api/v1"
	_ "github.com/tbxark/sphere/api/dash/v1"
	_ "github.com/tbxark/sphere/api/shared/v1"
)

// DO NOT DELETE THIS FILE

// @title sphere
// @version 1.0.0
// @description sphere api docs
// @accept json
// @produce json

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
// @description JWT token

// @security ApiKeyAuth []
