package main

import (
	"github.com/tbxark/go-base-api/cmd/app/app"
	boot2 "github.com/tbxark/go-base-api/pkg/utils/boot"
)

func main() {
	c := boot2.DefaultCommandConfigFlagsParser()
	err := boot2.Run(c, app.NewApplication)
	if err != nil {
		panic(err)
	}
}
