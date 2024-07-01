package cmd

import (
	"encoding/json"
	"github.com/tbxark/go-base-api/config"
	"github.com/tbxark/go-base-api/pkg/log"
	"os"

	"github.com/spf13/cobra"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Generate config file",
	Long:  `Generate a config file with default values.`,
	Run:   runConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.Flags().StringP("output", "o", "config_gen.json", "output file path")
	configCmd.Flags().StringP("database", "d", "mysql", "database type")
}

func runConfig(cmd *cobra.Command, args []string) {
	output := cmd.Flag("output").Value.String()
	conf := config.NewEmptyConfig()
	switch cmd.Flag("database").Value.String() {
	case "mysql":
		conf.Database.Type = "mysql"
		conf.Database.Path = "root:passwd@tcp(localhost:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s"
	case "sqlite":
		conf.Database.Type = "sqlite3"
		conf.Database.Path = "file:data.db?cache=shared&mode=rwc"
	}
	bytes, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		log.Fatalf("marshal config error: %v", err)
	}
	err = os.WriteFile(output, bytes, 0644)
	if err != nil {
		log.Fatalf("write config error: %v", err)
	}
}
