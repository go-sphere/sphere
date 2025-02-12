//go:build wireinject
// +build wireinject

package main

import (
	"github.com/TBXark/sphere/layout/internal"
	"github.com/TBXark/sphere/layout/internal/config"
	"github.com/TBXark/sphere/utils/boot"
	"github.com/google/wire"
)

func NewApplication(conf *config.Config) (*boot.Application, error) {
	wire.Build(internal.ProviderSet, wire.NewSet(newApplication))
	return &boot.Application{}, nil
}

func NewAPIApplication(conf *config.Config) (*boot.Application, error) {
	wire.Build(internal.ProviderSet, wire.NewSet(newAPIApplication))
	return &boot.Application{}, nil
}

func NewDashApplication(conf *config.Config) (*boot.Application, error) {
	wire.Build(internal.ProviderSet, wire.NewSet(newDashApplication))
	return &boot.Application{}, nil
}

func NewBotApplication(conf *config.Config) (*boot.Application, error) {
	wire.Build(internal.ProviderSet, wire.NewSet(newBotApplication))
	return &boot.Application{}, nil
}
