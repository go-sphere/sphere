package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tbxark/go-base-api/cmd/cli/app"
	"github.com/tbxark/go-base-api/internal/pkg/boot"
	"github.com/tbxark/go-base-api/pkg/log"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the application",
	Long:  `Start the application with configuration.`,
	Run:   runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringP("config", "c", "config.json", "config file path")
	startCmd.Flags().StringP("provider", "p", "", "config provider")
	startCmd.Flags().StringP("endpoint", "e", "", "config endpoint")
}

func runStart(cmd *cobra.Command, args []string) {
	conf, err := boot.LoadConfig(cmd.Flag("config").Value.String())
	if err != nil {
		log.Panicf("load config error: %v", err)
	}
	err = boot.Run(conf, app.NewApplication)
	if err != nil {
		log.Panicf("run application error: %v", err)
	}
}
