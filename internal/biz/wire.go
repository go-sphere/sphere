//go:build wireinject
// +build wireinject

package biz

import (
	"github.com/google/wire"
	"github.com/tbxark/go-base-api/internal/biz/api"
	"github.com/tbxark/go-base-api/internal/biz/bot"
	"github.com/tbxark/go-base-api/internal/biz/dash"
	"github.com/tbxark/go-base-api/internal/biz/task"
)

var ProviderSet = wire.NewSet(api.NewWebServer, dash.NewWebServer, bot.NewApp, task.NewInitialize, task.NewCleaner)
