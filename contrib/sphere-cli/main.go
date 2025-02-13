package main

import "log"

func main() {
	err := RunCommand(
		"sphere-cli",
		"A tool for managing sphere projects",
		map[string]*Command{
			"create": createProjectCommand(),
		},
	)
	if err != nil {
		log.Panicf("run command error: %v", err)
	}
}
