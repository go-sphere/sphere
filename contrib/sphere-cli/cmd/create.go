package cmd

import (
	"github.com/TBXark/sphere/contrib/sphere-cli/internal/create"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new Sphere project",
	Long:  `Create a new Sphere project with the specified name and optional template.`,
}

func init() {
	rootCmd.AddCommand(createCmd)

	flag := createCmd.Flags()
	name := flag.String("name", "", "Name of the new Sphere project")
	module := flag.String("module", "", "Go module name for the project (optional)")

	createCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if *name == "" {
			return cmd.Usage()
		}
		if *module == "" {
			module = name // Default to the project name if no module is specified
		}
		return create.Project(*name, *module)
	}
}
