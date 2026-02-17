package cmd

import (
	"fmt"

	"github.com/cjairm/rocketctl/internal/compose"
	"github.com/cjairm/rocketctl/internal/config"
	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec [service] [command...]",
	Short: "Execute a command in a running container",
	Long:  `Executes a command inside a running container.`,
	RunE:  runExec,
	Args:  cobra.MinimumNArgs(2),
}

func init() {
	rootCmd.AddCommand(execCmd)
}

func runExec(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	serviceName := args[0]
	if err := cfg.ValidateService(serviceName); err != nil {
		return err
	}

	// Compose container name format: <project>-<service>
	containerName := fmt.Sprintf("%s-%s", cfg.Project, serviceName)
	command := args[1:]

	return compose.Exec(containerName, command)
}
