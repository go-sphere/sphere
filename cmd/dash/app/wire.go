//go:build wireinject
// +build wireinject

package app

import (
	"github.com/google/wire"
	"github.com/tbxark/go-base-api/config"
	"github.com/tbxark/go-base-api/internal/biz"
	ipkg "github.com/tbxark/go-base-api/internal/pkg"
	"github.com/tbxark/go-base-api/internal/pkg/boot"
	"github.com/tbxark/go-base-api/pkg"
)

func NewDashApplication(conf *config.Config) (*boot.Application, error) {
	wire.Build(pkg.ProviderSet, ipkg.ProviderSet, biz.ProviderSet, config.ProviderSet, wire.NewSet(CreateApplication))
	return &boot.Application{}, nil
}
