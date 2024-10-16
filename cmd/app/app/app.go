package app

import (
	"github.com/tbxark/go-base-api/internal/biz/task"
	"github.com/tbxark/go-base-api/internal/server/api"
	"github.com/tbxark/go-base-api/internal/server/dash"
	"github.com/tbxark/go-base-api/pkg/utils/boot"
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
