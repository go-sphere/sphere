package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/TBXark/sphere/layout/internal/config"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test config file format",
	Long:  `Test config file format is correct.`,
}

func init() {
	rootCmd.AddCommand(testCmd)

	flag := testCmd.Flags()
	conf := flag.String("config", "config.json", "config file path")

	testCmd.RunE = func(cmd *cobra.Command, args []string) error {
		con, err := config.NewConfig(*conf)
		if err != nil {
			return err
		}
		bytes, err := json.MarshalIndent(con, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(bytes))
		return nil
	}
}
