//go:build wireinject
// +build wireinject

package app

import (
	"github.com/google/wire"
	"github.com/tbxark/go-base-api/config"
	"github.com/tbxark/go-base-api/internal/biz"
	"github.com/tbxark/go-base-api/internal/pkg/boot"
)

func NewBotApplication(conf *config.Config) (*boot.Application, error) {
	wire.Build(biz.ProviderSet, config.ProviderSet, wire.NewSet(CreateApplication))
	return &boot.Application{}, nil
}
