package app

import (
	"github.com/tbxark/go-base-api/internal/biz/dash"
	"github.com/tbxark/go-base-api/internal/biz/task"
	"github.com/tbxark/go-base-api/internal/pkg/boot"
)

func CreateApplication(dash *dash.Web, initialize *task.DashInitialize, cleaner *task.ConnectCleaner) *boot.Application {
	return boot.NewApplication(
		[]boot.Task{
			dash,
			initialize,
		},
		[]boot.Cleaner{
			cleaner,
		})
}
