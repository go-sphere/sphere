package cmd

import (
	"github.com/spf13/cobra"
	"os"
)

var rootCmd = &cobra.Command{
	Use:	"config",
	Short:	"Config Tools",
	Long:	`Config Tools is a set of tools for config operations.`,
	Run:	runRoot,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
}

func runRoot(cmd *cobra.Command, args []string) {
	_ = cmd.Usage()
}
