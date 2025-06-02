package cmd

import (
	"os"

	"github.com/TBXark/confstore"
	"github.com/TBXark/sphere/layout/internal/config"
	"github.com/TBXark/sphere/utils/safe"
	"github.com/spf13/cobra"
)

var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate config file",
	Long:  `Generate a config file with default values.`,
}

func init() {
	rootCmd.AddCommand(genCmd)

	flag := genCmd.Flags()
	output := flag.String("output", "config_gen.json", "output file path")
	database := flag.String("database", "sqlite", "database type")

	genCmd.RunE = func(*cobra.Command, []string) error {
		conf := config.NewEmptyConfig()
		switch *database {
		case "mysql":
			conf.Database.Type = "mysql"
			conf.Database.Path = "api:password@tcp(localhost:3306)/sphere?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s"
		case "sqlite":
			conf.Database.Type = "sqlite3"
			conf.Database.Path = "file:./var/data.db?cache=shared&mode=rwc"
		}
		file, err := os.Create(*output)
		if err != nil {
			return err
		}
		defer safe.IfErrorPresent("close file", file.Close)
		err = confstore.Save(*output, conf)
		if err != nil {
			return err
		}
		return nil
	}
}
