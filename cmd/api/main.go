package main

import (
	"github.com/tbxark/go-base-api/cmd/api/app"
	"github.com/tbxark/go-base-api/internal/pkg/boot"
)

func main() {
	c := boot.DefaultCommandConfigFlagsParser()
	err := boot.Run(c, app.NewAPIApplication)
	if err != nil {
		panic(err)
	}
}
