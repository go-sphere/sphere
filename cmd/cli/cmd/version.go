package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/tbxark/go-base-api/cmd/cli/config"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Long:  `Print the version number of the application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Version:", config.BuildVersion)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
