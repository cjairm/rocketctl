package cmd

import (
	"github.com/cjairm/rocketctl/internal/compose"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop the development environment",
	Long:  `Stops and removes containers created by docker compose.`,
	RunE:  runDown,
}

func init() {
	rootCmd.AddCommand(downCmd)
}

func runDown(cmd *cobra.Command, args []string) error {
	return compose.Down("docker-compose.yml")
}
