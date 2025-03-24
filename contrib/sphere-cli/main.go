package main

import (
	"log"

	"github.com/TBXark/sphere/contrib/sphere-cli/cmd/create"
	"github.com/TBXark/sphere/contrib/sphere-cli/cmd/rename"
	"github.com/TBXark/sphere/contrib/sphere-cli/internal/command"
)

func main() {
	err := command.RunCommand(
		"sphere-cli",
		"A tool for managing sphere projects",
		[]*command.Command{
			create.NewCommand(),
			rename.NewCommand(),
		},
	)
	if err != nil {
		log.Panicf("run command error: %v", err)
	}
}
