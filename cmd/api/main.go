package main

import (
	"github.com/tbxark/go-base-api/cmd/api/app"
	boot2 "github.com/tbxark/go-base-api/pkg/utils/boot"
)

func main() {
	c := boot2.DefaultCommandConfigFlagsParser()
	err := boot2.Run(c, app.NewAPIApplication)
	if err != nil {
		panic(err)
	}
}
