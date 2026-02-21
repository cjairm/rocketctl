package cmd

import (
	"fmt"

	"github.com/cjairm/rocketctl/internal/compose"
	"github.com/spf13/cobra"
)

var (
	downProdFlag bool
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop services",
	Long:  `Stops and removes containers created by docker compose. Use --prod to stop production/test containers.`,
	RunE:  runDown,
}

func init() {
	downCmd.Flags().BoolVar(&downProdFlag, "prod", false, "Stop production/test containers (docker-compose.prod.yml)")
	rootCmd.AddCommand(downCmd)
}

func runDown(cmd *cobra.Command, args []string) error {
	composeFile := "docker-compose.yml"
	env := "development"

	if downProdFlag {
		composeFile = "docker-compose.prod.yml"
		env = "production/test"
	}

	fmt.Printf("Stopping %s environment...\n", env)
	return compose.Down(composeFile)
}
