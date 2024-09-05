package main

import (
	"github.com/tbxark/go-base-api/cmd/api/app"
	"github.com/tbxark/go-base-api/internal/pkg/boot"
)

// @securityDefinitions.apikey	ApiKeyAuth
// @in							header
// @name						Authorization
// @description				    JWT token
func main() {
	c := boot.DefaultCommandConfigFlagsParser()
	err := boot.Run(c, nil, app.NewAPIApplication)
	if err != nil {
		panic(err)
	}
}
