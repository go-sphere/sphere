package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "backend",
	Short: "Run the backend server",
	Long:  `Run the backend server to provide the API and dashboard service.`,
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
