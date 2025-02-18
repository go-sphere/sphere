package cmd

import (
	"encoding/json"
	"github.com/TBXark/sphere/contrib/json-gen-proto/gen"
	"github.com/TBXark/sphere/layout/internal/config"
	"github.com/spf13/cobra"
	"log"
	"os"
)

// protoCmd represents the config command
var protoCmd = &cobra.Command{
	Use:   "proto",
	Short: "Generate config proto file",
	Long:  `Generate proto definition file.`,
	Run:   runConfig,
}

func init() {
	rootCmd.AddCommand(protoCmd)
	protoCmd.Flags().StringP("output", "o", "config.proto", "output file path")
	protoCmd.Flags().StringP("package", "p", "config", "package name")
}

func runProto(cmd *cobra.Command, args []string) {
	output := cmd.Flag("output").Value.String()
	pkgName := cmd.Flag("package").Value.String()
	conf := config.NewEmptyConfig()
	bytes, err := json.Marshal(conf)
	if err != nil {
		log.Fatalf("marshal error: %v", err)
	}
	protoStr, err := gen.Generate(pkgName, "Config", bytes)
	if err != nil {
		log.Fatalf("generate proto error: %v", err)
	}
	err = os.WriteFile(output, []byte(protoStr), 0644)
	if err != nil {
		log.Fatalf("write proto file error: %v", err)
	}
}
