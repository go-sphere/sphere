package biz

import (
	"github.com/google/wire"
	"github.com/tbxark/go-base-api/internal/biz/bot"
	"github.com/tbxark/go-base-api/internal/biz/task"
)

var ProviderSet = wire.NewSet(bot.NewApp, task.NewDashInitialize, task.NewConnectCleaner)
