package rename

import (
	"flag"

	"github.com/TBXark/sphere/contrib/sphere-cli/internal/command"
	"github.com/TBXark/sphere/contrib/sphere-cli/internal/renamer"
)

func NewCommand() *command.Command {
	fs := flag.NewFlagSet("rename", flag.ExitOnError)
	oldMod := fs.String("old", "", "old go mod name")
	newMod := fs.String("new", "", "new go module name")
	target := fs.String("target", ".", "target directory")
	return command.NewCommand(fs, func() error {
		if *oldMod == "" || *newMod == "" {
			fs.Usage()
			return nil
		}
		return renamer.RenameDirModule(*oldMod, *newMod, *target)
	})
}
