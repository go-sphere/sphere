//go:build wireinject
// +build wireinject

package configs

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	wire.FieldsOf(new(*Config), "Log", "Database", "Dash", "API", "Storage", "Bot", "WxMini"),
)
