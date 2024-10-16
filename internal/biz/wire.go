package biz

import (
	"github.com/google/wire"
	"github.com/tbxark/sphere/internal/biz/bot"
	"github.com/tbxark/sphere/internal/biz/task"
)

var ProviderSet = wire.NewSet(bot.NewApp, task.NewDashInitialize, task.NewConnectCleaner)
