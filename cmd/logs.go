package cmd

import (
	"fmt"

	"github.com/cjairm/rocketctl/internal/compose"
	"github.com/cjairm/rocketctl/internal/config"
	"github.com/spf13/cobra"
)

var (
	followFlag   bool
	logsProdFlag bool
)

var logsCmd = &cobra.Command{
	Use:   "logs [service]",
	Short: "Show logs for a service",
	Long: `Shows docker compose logs on this machine, from docker-compose.yml or, with
--prod, docker-compose.prod.yml. It does not read logs from the deploy server.

The optional argument is a rocket.yaml service; it maps to the compose service
<project>-<service>. Without it, logs from every service are shown.`,
	Example: `  rocketctl logs                 # all dev services
  rocketctl logs api -f          # follow one service
  rocketctl logs --prod api -f   # the stack started by 'up --prod'`,
	RunE: runLogs,
}

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.Flags().BoolVarP(&followFlag, "follow", "f", false, "Follow log output")
	logsCmd.Flags().BoolVar(&logsProdFlag, "prod", false, "View production/test container logs (docker-compose.prod.yml)")
}

func runLogs(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Determine which compose file to use
	composeFile := "docker-compose.yml"
	if logsProdFlag {
		composeFile = "docker-compose.prod.yml"
	}

	var service string
	if len(args) > 0 {
		serviceName := args[0]
		if err := cfg.ValidateService(serviceName); err != nil {
			return err
		}
		// Compose service name format: <project>-<service>
		service = fmt.Sprintf("%s-%s", cfg.Project, serviceName)
	}

	return compose.Logs(composeFile, service, followFlag)
}
