package config

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	wire.FieldsOf(new(*Config), "Log", "Database", "Dash", "API", "Docs", "Storage", "Bot", "WxMini"),
)
