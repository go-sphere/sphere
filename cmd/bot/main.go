package main

import (
	"github.com/gin-gonic/gin"
	"github.com/tbxark/go-base-api/cmd/bot/app"
	"github.com/tbxark/go-base-api/config"
	"github.com/tbxark/go-base-api/internal/pkg/boot"
)

func main() {
	c := boot.DefaultCommandConfigFlagsParser()
	err := boot.Run(c, func(c *config.Config) {
		gin.SetMode(c.System.GinMode)
	}, app.NewBotApplication)
	if err != nil {
		panic(err)
	}
}
