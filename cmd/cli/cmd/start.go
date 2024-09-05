package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tbxark/go-base-api/cmd/cli/app"
	"github.com/tbxark/go-base-api/config"
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
}

func runStart(cmd *cobra.Command, args []string) {
	cfg := cmd.Flag("config").Value.String()
	conf, err := config.LoadConfig(cfg)
	if err != nil {
		log.Panicf("load config error: %v", err)
	}
	err = app.Run(conf)
	if err != nil {
		log.Panicf("run application error: %v", err)
	}
}
