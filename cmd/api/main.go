package main

import (
	"github.com/tbxark/sphere/internal/biz/task"
	"github.com/tbxark/sphere/internal/pkg/app"
	"github.com/tbxark/sphere/internal/server/api"
	"github.com/tbxark/sphere/pkg/utils/boot"
)

func main() {
	app.Execute(NewAPIApplication)
}

func newApplication(dash *api.Web, initialize *task.DashInitialize, cleaner *task.ConnectCleaner) *boot.Application {
	return boot.NewApplication(
		[]boot.Task{
			dash,
			initialize,
		},
		[]boot.Cleaner{
			dash,
			cleaner,
		})
}
