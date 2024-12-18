package biz

import (
	"github.com/TBXark/sphere/internal/biz/task"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(task.NewDashInitialize, task.NewConnectCleaner)
