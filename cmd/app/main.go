package main

import (
	"github.com/TBXark/sphere/internal/biz/task"
	"github.com/TBXark/sphere/internal/pkg/app"
	"github.com/TBXark/sphere/internal/server/api"
	"github.com/TBXark/sphere/internal/server/dash"
	"github.com/TBXark/sphere/internal/server/docs"
	"github.com/TBXark/sphere/pkg/utils/boot"
)

func main() {
	app.Execute(NewApplication)
}

func newApplication(dash *dash.Web, api *api.Web, docs *docs.Web, initialize *task.DashInitialize, cleaner *task.ConnectCleaner) *boot.Application {
	return boot.NewApplication(
		dash,
		api,
		docs,
		initialize,
		cleaner,
	)
}
