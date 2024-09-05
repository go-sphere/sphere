package main

import (
	"github.com/tbxark/go-base-api/cmd/api/app"
	"github.com/tbxark/go-base-api/internal/pkg/boot"
	"github.com/tbxark/go-base-api/pkg/log"
)

// @securityDefinitions.apikey	ApiKeyAuth
// @in							header
// @name						Authorization
// @description				    JWT token
func main() {
	if err := boot.RunWithConfig("dash", app.NewApplication); err != nil {
		log.Errorw("run api error", "error", err)
	}
}
