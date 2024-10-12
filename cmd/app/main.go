package main

import (
	"github.com/tbxark/go-base-api/cmd/app/app"
	"github.com/tbxark/go-base-api/internal/pkg/boot"
)

func main() {
	c := boot.DefaultCommandConfigFlagsParser()
	err := boot.Run(c, app.NewApplication)
	if err != nil {
		panic(err)
	}
}
