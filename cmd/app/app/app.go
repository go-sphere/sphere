package app

import (
	"github.com/tbxark/sphere/internal/biz/task"
	"github.com/tbxark/sphere/internal/server/api"
	"github.com/tbxark/sphere/internal/server/dash"
	"github.com/tbxark/sphere/pkg/utils/boot"
)

func CreateApplication(dash *dash.Web, api *api.Web, initialize *task.DashInitialize, cleaner *task.ConnectCleaner) *boot.Application {
	return boot.NewApplication(
		[]boot.Task{
			dash,
			api,
			initialize,
		},
		[]boot.Cleaner{
			cleaner,
		})
}
