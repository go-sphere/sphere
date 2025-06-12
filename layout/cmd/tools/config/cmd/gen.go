package cmd

import (
	"os"
	"time"

	"github.com/TBXark/confstore"
	"github.com/TBXark/sphere/core/safe"
	"github.com/TBXark/sphere/layout/internal/config"
	"github.com/go-sql-driver/mysql"
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
			dsn := mysql.Config{
				User:                 "example",
				Passwd:               "password",
				Net:                  "tcp",
				Addr:                 "127.0.0.1:3306",
				DBName:               "sphere",
				Loc:                  time.Local,
				Timeout:              time.Second * 10,
				ParseTime:            true,
				AllowNativePasswords: true,
			}
			_ = dsn.Apply(mysql.Charset("utf8mb4", "utf8mb4_unicode_ci"))
			conf.Database.Path = dsn.FormatDSN()
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
