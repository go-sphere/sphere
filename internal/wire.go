package internal

import (
	"github.com/google/wire"
	"github.com/tbxark/go-base-api/internal/biz"
	"github.com/tbxark/go-base-api/internal/pkg"
	"github.com/tbxark/go-base-api/internal/server"
)

var ProviderSet = wire.NewSet(server.ProviderSet, pkg.ProviderSet, biz.ProviderSet)
