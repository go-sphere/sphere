package internal

import (
	"github.com/google/wire"
	"github.com/tbxark/sphere/internal/biz"
	"github.com/tbxark/sphere/internal/pkg"
	"github.com/tbxark/sphere/internal/server"
)

var ProviderSet = wire.NewSet(server.ProviderSet, pkg.ProviderSet, biz.ProviderSet)
