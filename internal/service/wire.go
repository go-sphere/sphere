package service

import (
	"github.com/google/wire"
	"github.com/tbxark/sphere/internal/service/api"
	"github.com/tbxark/sphere/internal/service/bot"
	"github.com/tbxark/sphere/internal/service/dash"
)

var ProviderSet = wire.NewSet(api.NewService, dash.NewService, bot.NewService)
