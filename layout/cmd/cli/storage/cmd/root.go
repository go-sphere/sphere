package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "storage",
	Short: "storage Tools",
	Long:  `storage Tools is a set of tools for storage operations.`,
	Run:   runRoot,
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
