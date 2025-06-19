package main

import (
	"github.com/TBXark/sphere/core/boot"
	"github.com/TBXark/sphere/layout/internal/biz/task/conncleaner"
	"github.com/TBXark/sphere/layout/internal/biz/task/dashinit"
	"github.com/TBXark/sphere/layout/internal/server/api"
	"github.com/TBXark/sphere/layout/internal/server/bot"
	"github.com/TBXark/sphere/layout/internal/server/dash"
	"github.com/TBXark/sphere/server/service/file"
)

func newApplication(
	dash *dash.Web,
	api *api.Web,
	bot *bot.Bot,
	file *file.Web,
	initialize *dashinit.DashInitialize,
	cleaner *conncleaner.ConnectCleaner,
) *boot.Application {
	return boot.NewApplication(
		dash,
		api,
		bot,
		file,
		initialize,
		cleaner,
	)
}
