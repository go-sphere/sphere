//go:build wireinject
// +build wireinject

package app

import (
	"github.com/google/wire"
	"github.com/tbxark/go-base-api/configs"
	"github.com/tbxark/go-base-api/internal"
	"github.com/tbxark/go-base-api/pkg/utils/boot"
)

func NewBotApplication(conf *configs.Config) (*boot.Application, error) {
	wire.Build(configs.ProviderSet, internal.ProviderSet, wire.NewSet(CreateApplication))
	return &boot.Application{}, nil
}
