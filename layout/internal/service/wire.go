package service

import (
	"github.com/TBXark/sphere/layout/internal/service/api"
	"github.com/TBXark/sphere/layout/internal/service/bot"
	"github.com/TBXark/sphere/layout/internal/service/dash"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(api.NewService, dash.NewService, bot.NewService)
