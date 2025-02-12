//go:generate swag init --output ./swagger/api --tags api.v1,shared.v1 --instanceName API -g docs.go --parseDependency
//go:generate swag init --output ./swagger/dash --tags dash.v1,shared.v1 --instanceName Dash -g docs.go --parseDependency

//go:generate npx swagger-typescript-api -p ./swagger/api/API_swagger.json -o ./swagger/api/typescript --modular --responses --extract-response-body --extract-response-error
//go:generate npx swagger-typescript-api -p ./swagger/dash/Dash_swagger.json -o ./swagger/dash/typescript --modular --responses --extract-response-body --extract-response-error

package layout

import (
	_ "github.com/TBXark/sphere/layout/api/api/v1"
	_ "github.com/TBXark/sphere/layout/api/dash/v1"
	_ "github.com/TBXark/sphere/layout/api/shared/v1"
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
