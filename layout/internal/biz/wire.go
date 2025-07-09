package biz

import (
	"github.com/TBXark/sphere/layout/internal/biz/cron/testutils"
	"github.com/TBXark/sphere/layout/internal/biz/task/conncleaner"
	"github.com/TBXark/sphere/layout/internal/biz/task/dashinit"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	dashinit.NewDashInitialize,
	conncleaner.NewConnectCleaner,
	testutils.NewAutoClose,
)
