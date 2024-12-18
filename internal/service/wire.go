package service

import (
	"github.com/TBXark/sphere/internal/service/api"
	"github.com/TBXark/sphere/internal/service/bot"
	"github.com/TBXark/sphere/internal/service/dash"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(api.NewService, dash.NewService, bot.NewService)
