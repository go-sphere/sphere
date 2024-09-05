//go:build wireinject
// +build wireinject

package config

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	wire.FieldsOf(new(*Config), "System", "Log", "Database", "Dash", "API", "CDN", "Bot", "WxMini"),
)
