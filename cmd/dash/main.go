package main

import (
	"github.com/TBXark/sphere/internal/biz/task"
	"github.com/TBXark/sphere/internal/pkg/app"
	"github.com/TBXark/sphere/internal/server/dash"
	"github.com/TBXark/sphere/pkg/utils/boot"
)

func main() {
	app.Execute(NewDashApplication)
}

func newApplication(dash *dash.Web, cleaner *task.ConnectCleaner, initDash *task.DashInitialize) *boot.Application {
	return boot.NewApplication(
		initDash,
		dash,
		cleaner,
	)
}
