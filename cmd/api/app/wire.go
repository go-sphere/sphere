//go:build wireinject
// +build wireinject

package app

import (
	"github.com/google/wire"
	"github.com/tbxark/go-base-api/config"
	"github.com/tbxark/go-base-api/internal/biz/api"
	ipkg "github.com/tbxark/go-base-api/internal/pkg"
	"github.com/tbxark/go-base-api/pkg"
)

func NewApplication(cfg *config.Config) (*api.Web, error) {
	wire.Build(pkg.ProviderSet, ipkg.ProviderSet, config.ProviderSet, wire.NewSet(api.NewWebServer))
	return &api.Web{}, nil
}
