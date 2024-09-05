package main

import (
	"github.com/tbxark/go-base-api/cmd/bot/app"
	"github.com/tbxark/go-base-api/internal/pkg/boot"
	"github.com/tbxark/go-base-api/pkg/log"
)

func main() {
	if err := boot.RunWithConfig("dash", app.NewApplication); err != nil {
		log.Errorw("run bot error", "error", err)
	}
}
