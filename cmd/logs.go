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
	Long:  `Shows logs for a service using docker compose. Use --prod to view production/test container logs.`,
	RunE:  runLogs,
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
