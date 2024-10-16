//go:build wireinject
// +build wireinject

package app

import (
	"github.com/google/wire"
	"github.com/tbxark/sphere/configs"
	"github.com/tbxark/sphere/internal"
	"github.com/tbxark/sphere/pkg"
	"github.com/tbxark/sphere/pkg/utils/boot"
)

func NewDashApplication(conf *configs.Config) (*boot.Application, error) {
	wire.Build(configs.ProviderSet, pkg.ProviderSet, internal.ProviderSet, wire.NewSet(CreateApplication))
	return &boot.Application{}, nil
}
