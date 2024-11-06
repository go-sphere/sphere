//go:build wireinject
// +build wireinject

package main

import (
	"github.com/google/wire"
	"github.com/tbxark/sphere/internal"
	"github.com/tbxark/sphere/internal/config"
	"github.com/tbxark/sphere/pkg/utils/boot"
)

func NewAPIApplication(conf *config.Config) (*boot.Application, error) {
	wire.Build(internal.ProviderSet, wire.NewSet(newApplication))
	return &boot.Application{}, nil
}
