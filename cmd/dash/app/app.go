package app

import (
	"github.com/tbxark/go-base-api/internal/biz/api"
	"github.com/tbxark/go-base-api/internal/biz/task"
	"github.com/tbxark/go-base-api/internal/pkg/boot"
)

func CreateApplication(dash *api.Web, cleaner *task.ConnectCleaner) *boot.Application {
	return boot.NewApplication(
		[]boot.Task{
			dash,
		},
		[]boot.Cleaner{
			cleaner,
		})
}
