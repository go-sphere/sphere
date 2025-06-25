package cmd

import (
	"github.com/TBXark/sphere/cmd/sphere-cli/internal/renamer"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:   "rename",
	Short: "Rename Go module in a directory",
	Long:  `Rename the Go module in the specified directory from old to new name.`,
}

func init() {
	rootCmd.AddCommand(renameCmd)

	flag := renameCmd.Flags()
	oldMod := flag.String("old", "", "Old Go module name")
	newMod := flag.String("new", "", "New Go module name")
	target := flag.String("target", ".", "Target directory to rename the module in")

	renameCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if *oldMod == "" || *newMod == "" {
			return cmd.Usage()
		}
		return renamer.RenameDirModule(*oldMod, *newMod, *target)
	}
}
