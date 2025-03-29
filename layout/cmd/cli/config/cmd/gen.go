package cmd

import (
	"log"
	"os"

	"github.com/TBXark/confstore"
	"github.com/TBXark/sphere/layout/internal/config"
	"github.com/TBXark/sphere/utils/safe"
	"github.com/spf13/cobra"
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

type Encoder interface {
	Encode(v interface{}) error
}

func runConfig(cmd *cobra.Command, args []string) {
	output := cmd.Flag("output").Value.String()
	conf := config.NewEmptyConfig()
	switch cmd.Flag("database").Value.String() {
	case "mysql":
		conf.Database.Type = "mysql"
		conf.Database.Path = "api:password@tcp(localhost:3306)/sphere?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s"
	case "sqlite":
		conf.Database.Type = "sqlite3"
		conf.Database.Path = "file:./var/data.db?cache=shared&mode=rwc"
	}
	file, err := os.Create(output)
	if err != nil {
		log.Fatalf("create file error: %v", err)
	}
	defer safe.IfErrorPresent("close file", file.Close)
	err = confstore.Save(output, conf)
	if err != nil {
		log.Fatalf("encode error: %v", err)
	}
}
