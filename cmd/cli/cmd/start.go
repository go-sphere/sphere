package cmd

import (
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"github.com/tbxark/go-base-api/cmd/cli/app"
	"github.com/tbxark/go-base-api/config"
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

func loadConfig(cmd *cobra.Command) (*config.Config, error) {
	path := cmd.Flag("config").Value.String()
	provider := cmd.Flag("provider").Value.String()
	endpoint := cmd.Flag("endpoint").Value.String()
	if provider == "" {
		return config.LoadLocalConfig(path)
	}
	return config.LoadRemoteConfig(provider, endpoint, path)
}

func runStart(cmd *cobra.Command, args []string) {
	conf, err := loadConfig(cmd)
	if err != nil {
		log.Panicf("load config error: %v", err)
	}
	err = boot.Run(conf, func(c *config.Config) {
		gin.SetMode(c.System.GinMode)
	}, app.NewApplication)
	if err != nil {
		log.Panicf("run application error: %v", err)
	}
}
