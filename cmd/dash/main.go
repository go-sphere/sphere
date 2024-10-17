package main

import (
	"github.com/tbxark/sphere/internal/biz/task"
	"github.com/tbxark/sphere/internal/pkg/app"
	"github.com/tbxark/sphere/internal/server/dash"
	"github.com/tbxark/sphere/pkg/utils/boot"
)

func main() {
	app.Execute(NewDashApplication)
}

func newApplication(dash *dash.Web, cleaner *task.ConnectCleaner) *boot.Application {
	return boot.NewApplication(
		[]boot.Task{
			dash,
		},
		[]boot.Cleaner{
			cleaner,
		})
}
