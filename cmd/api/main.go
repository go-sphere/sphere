package main

import (
	"github.com/gin-gonic/gin"
	"github.com/tbxark/go-base-api/cmd/api/app"
	"github.com/tbxark/go-base-api/config"
	"github.com/tbxark/go-base-api/internal/pkg/boot"
)

// @securityDefinitions.apikey	ApiKeyAuth
// @in							header
// @name						Authorization
// @description				    JWT token
func main() {
	c := boot.DefaultCommandConfigFlagsParser()
	err := boot.Run(c, func(c *config.Config) {
		gin.SetMode(c.System.GinMode)
	}, app.NewAPIApplication)
	if err != nil {
		panic(err)
	}
}
