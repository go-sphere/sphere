package biz

import (
	"github.com/google/wire"
	"github.com/tbxark/sphere/internal/biz/task"
)

var ProviderSet = wire.NewSet(task.NewDashInitialize, task.NewConnectCleaner)
