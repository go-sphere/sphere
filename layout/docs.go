//go:generate go tool swag init --output ./swagger/api --tags api.v1,shared.v1 --instanceName API -g docs.go --parseDependency
//go:generate go tool swag init --output ./swagger/dash --tags dash.v1,shared.v1 --instanceName Dash -g docs.go --parseDependency

package layout

import (
	_ "github.com/TBXark/sphere/layout/api/api/v1"
	_ "github.com/TBXark/sphere/layout/api/dash/v1"
	_ "github.com/TBXark/sphere/layout/api/shared/v1"
)

// DO NOT DELETE THIS FILE

// This file needs to be placed in the project root directory to index all modules under this mod.
// This file is used to generate API documentation, using the Swag tool
// If you have API documentation located in other directories, add the corresponding package path in import.
// Here is the global API documentation configuration.

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
