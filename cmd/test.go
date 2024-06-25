package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/tbxark/go-base-api/config"
	"github.com/tbxark/go-base-api/pkg/log"
)

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test config file format",
	Long:  `Test config file format is correct.`,
	Run:   runTest,
}

func init() {
	rootCmd.AddCommand(testCmd)
	testCmd.Flags().StringP("config", "c", "config.json", "config file path")
}

func runTest(cmd *cobra.Command, args []string) {
	cfg := cmd.Flag("config").Value.String()
	con, err := config.LoadConfig(cfg)
	if err != nil {
		log.Panicf("load config error: %v", err)
	}
	bytes, err := json.MarshalIndent(con, "", "  ")
	if err != nil {
		log.Fatalf("marshal config error: %v", err)
	}
	fmt.Println(string(bytes))
}
