//go:build wireinject
// +build wireinject

package main

import (
	"github.com/TBXark/sphere/internal"
	"github.com/TBXark/sphere/internal/config"
	"github.com/TBXark/sphere/pkg/utils/boot"
	"github.com/google/wire"
)

func NewApplication(conf *config.Config) (*boot.Application, error) {
	wire.Build(internal.ProviderSet, wire.NewSet(newApplication))
	return &boot.Application{}, nil
}
