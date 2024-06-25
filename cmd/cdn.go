package cmd

import (
	"github.com/spf13/cobra"
)

// cdnCmd represents the cdn command
var cdnCmd = &cobra.Command{
	Use:   "cdn",
	Short: "CDN Tools",
	Long:  `CDN Tools is a set of tools for CDN operations,`,
	Run:   runCDN,
}

func init() {
	rootCmd.AddCommand(cdnCmd)
}

func runCDN(cmd *cobra.Command, args []string) {
	_ = cmd.Usage()
}
