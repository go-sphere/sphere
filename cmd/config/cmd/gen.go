package cmd

import (
	"encoding/json"
	"github.com/spf13/cobra"
	"github.com/tbxark/go-base-api/configs"
	"github.com/tbxark/go-base-api/pkg/log"
	"os"
)

// genCmd represents the config command
var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate config file",
	Long:  `Generate a config file with default values.`,
	Run:   runConfig,
}

func init() {
	rootCmd.AddCommand(genCmd)
	genCmd.Flags().StringP("output", "o", "config_gen.json", "output file path")
	genCmd.Flags().StringP("database", "d", "sqlite", "database type")
}

func runConfig(cmd *cobra.Command, args []string) {
	output := cmd.Flag("output").Value.String()
	conf := configs.NewEmptyConfig()
	switch cmd.Flag("database").Value.String() {
	case "mysql":
		conf.Database.Type = "mysql"
		conf.Database.Path = "api:password@tcp(localhost:3306)/go-base?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s"
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
