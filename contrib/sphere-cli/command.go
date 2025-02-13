package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

type Command struct {
	Name       string
	Usage      string
	FlagSet    *flag.FlagSet
	HandleFunc func() error
}

func NewCommand(fs *flag.FlagSet, handleFunc func() error) *Command {
	cmd := &Command{
		Name:    fs.Name(),
		FlagSet: fs,
	}
	help := cmd.FlagSet.Bool("help", false, "show help")
	cmd.HandleFunc = func() error {
		if *help {
			fs.Usage()
			return nil
		}
		return handleFunc()
	}
	return cmd
}

func RunCommand(name, desc string, commands map[string]*Command) error {
	printDefaults := func() {
		fmt.Printf("Usage: %s <command> [options]\n\n", name)
		fmt.Printf("%s\n", desc)
		for name, cmd := range commands {
			fmt.Printf("%s Command:\n", strings.Title(name))
			cmd.FlagSet.PrintDefaults()
		}
	}

	if len(os.Args) < 2 {
		printDefaults()
		return nil
	}

	cmd, exists := commands[os.Args[1]]
	if !exists {
		printDefaults()
		return nil
	}

	if err := cmd.FlagSet.Parse(os.Args[2:]); err != nil {
		return err
	}

	if err := cmd.HandleFunc(); err != nil {
		return err
	}
	return nil
}
