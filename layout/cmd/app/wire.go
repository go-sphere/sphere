//go:build wireinject
// +build wireinject

package main

import (
	"github.com/TBXark/sphere/core/boot"
	"github.com/TBXark/sphere/layout/internal"
	"github.com/TBXark/sphere/layout/internal/config"
	"github.com/google/wire"
)

func NewApplication(conf *config.Config) (*boot.Application, error) {
	wire.Build(internal.ProviderSet, wire.NewSet(newApplication))
	return &boot.Application{}, nil
}
