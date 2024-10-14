//go:build wireinject
// +build wireinject

package app

import (
	"github.com/google/wire"
	"github.com/tbxark/go-base-api/configs"
	"github.com/tbxark/go-base-api/internal/biz"
	"github.com/tbxark/go-base-api/internal/pkg/boot"
)

func NewBotApplication(conf *configs.Config) (*boot.Application, error) {
	wire.Build(biz.ProviderSet, configs.ProviderSet, wire.NewSet(CreateApplication))
	return &boot.Application{}, nil
}
