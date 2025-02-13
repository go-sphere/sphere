package command

import (
	"flag"
	"fmt"
	"os"
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

func RunCommand(name, desc string, commands []*Command) error {
	printDefaults := func() {
		fmt.Printf("Usage: %s <command> [options]\n\n", name)
		fmt.Printf("%s\n", desc)
		for _, cmd := range commands {
			fmt.Printf("%s Command:\n", cmd.Name)
			cmd.FlagSet.PrintDefaults()
		}
	}

	if len(os.Args) < 2 {
		printDefaults()
		return nil
	}

	for _, cmd := range commands {
		if cmd.Name != os.Args[1] {
			continue
		}
		if err := cmd.FlagSet.Parse(os.Args[2:]); err != nil {
			return err
		}
		if err := cmd.HandleFunc(); err != nil {
			return err
		}
		return nil
	}

	printDefaults()
	return nil
}
