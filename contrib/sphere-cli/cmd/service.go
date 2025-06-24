package cmd

import (
	"github.com/TBXark/sphere/contrib/sphere-cli/internal/service"
	"github.com/spf13/cobra"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Generate service code",
	Long:  `Generate service code for Sphere projects, including service interfaces and implementations.`,
}

var serviceProtoCmd = &cobra.Command{
	Use:   "proto",
	Short: "Generate service proto code",
	Long:  `Generate service proto code for Sphere projects, including proto definitions and gRPC service implementations.`,
}

var serviceGolangCmd = &cobra.Command{
	Use:   "golang",
	Short: "Generate service Golang code",
	Long:  `Generate service Golang code for Sphere projects, including service interfaces and implementations in Go.`,
}

func init() {
	rootCmd.AddCommand(serviceCmd)
	serviceCmd.AddCommand(serviceProtoCmd)
	serviceCmd.AddCommand(serviceGolangCmd)

	{
		flag := serviceProtoCmd.Flags()
		name := flag.String("name", "", "Name of the service")
		pkg := flag.String("package", "dash.v1", "Package name for the generated proto code")
		serviceProtoCmd.RunE = func(cmd *cobra.Command, args []string) error {
			if *name == "" || *pkg == "" {
				return cmd.Usage()
			}
			text, err := service.GenServiceProto(*name, *pkg)
			if err != nil {
				return err
			}
			cmd.Println(text)
			return nil
		}
	}
	{
		flag := serviceGolangCmd.Flags()
		name := flag.String("name", "", "Name of the service")
		pkg := flag.String("package", "dash.v1", "Package name for the generated Go code")
		mod := flag.String("mod", "github.com/TBXark/sphere/layout", "Go module path for the generated code")
		serviceGolangCmd.RunE = func(cmd *cobra.Command, args []string) error {
			if *name == "" || *pkg == "" {
				return cmd.Usage()
			}
			text, err := service.GenServiceGolang(*name, *pkg, *mod)
			if err != nil {
				return err
			}
			cmd.Println(text)
			return nil
		}
	}
}
