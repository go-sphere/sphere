package config

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	wire.FieldsOf(new(*Config), "Environments", "Log", "Database", "Dash", "API", "File", "Docs", "Storage", "Bot", "WxMini"),
)
