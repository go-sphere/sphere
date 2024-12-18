package main

import (
	"github.com/TBXark/sphere/internal/biz/task"
	"github.com/TBXark/sphere/internal/pkg/app"
	"github.com/TBXark/sphere/internal/server/api"
	"github.com/TBXark/sphere/pkg/utils/boot"
)

func main() {
	app.Execute(NewAPIApplication)
}

func newApplication(dash *api.Web, initialize *task.DashInitialize, cleaner *task.ConnectCleaner) *boot.Application {
	return boot.NewApplication(
		dash,
		initialize,
		cleaner,
	)
}
