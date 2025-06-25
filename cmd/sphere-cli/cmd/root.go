package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "sphere-cli",
	Short: "A tool for managing sphere projects",
	Long:  `Sphere CLI is a command-line tool designed to help you manage Sphere projects efficiently.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Usage()
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
