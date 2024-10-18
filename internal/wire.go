package internal

import (
	"github.com/google/wire"
	"github.com/tbxark/sphere/internal/biz"
	"github.com/tbxark/sphere/internal/pkg"
	"github.com/tbxark/sphere/internal/server"
	"github.com/tbxark/sphere/internal/service"
)

var ProviderSet = wire.NewSet(server.ProviderSet, service.ProviderSet, pkg.ProviderSet, biz.ProviderSet)
