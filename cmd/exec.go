package cmd

import (
	"github.com/cjairm/rocketctl/internal/compose"
	"github.com/cjairm/rocketctl/internal/config"
	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec [service] [command...]",
	Short: "Execute a command in a running container",
	Long: `Runs a command (docker exec -it) in the container <project>-<service>
on this machine. The service is a rocket.yaml name and is always required, even
in a single-service repo. When stdin is not a terminal (a pipe, CI, cron) it
runs without a TTY (docker exec -i).

Put '--' before a command that has its own flags, so rocketctl doesn't read them.`,
	Example: `  rocketctl exec api bash
  rocketctl exec api -- ls -la /app
  rocketctl exec web -- sh -c 'env | sort'
  echo 'select 1' | rocketctl exec db -- psql -U app`,
	RunE: runExec,
	Args: cobra.MinimumNArgs(2),
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
	containerName := config.ComposeServiceName(cfg.Project, serviceName)
	command := args[1:]

	return compose.Exec(containerName, command)
}
