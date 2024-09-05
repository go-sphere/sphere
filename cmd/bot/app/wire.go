//go:build wireinject
// +build wireinject

package app

import (
	"github.com/google/wire"
	"github.com/tbxark/go-base-api/config"
	"github.com/tbxark/go-base-api/internal/biz/bot"
)

func NewApplication(cfg *config.Config) (*bot.App, error) {
	wire.Build(config.ProviderSet, wire.NewSet(bot.NewApp))
	return &bot.App{}, nil
}
