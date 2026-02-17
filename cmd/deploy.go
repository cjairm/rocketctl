package cmd

import (
	"github.com/cjairm/rocketctl/internal/compose"
	"github.com/cjairm/rocketctl/internal/config"
	"github.com/cjairm/rocketctl/internal/registry"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy services on production",
	Long:  `Authenticates with the registry, pulls latest images, and restarts services using docker-compose.prod.yml.`,
	RunE:  runDeploy,
}

func init() {
	rootCmd.AddCommand(deployCmd)
}

func runDeploy(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Authenticate with ECR
	if err := registry.LoginECR(cfg.Registry, cfg.Region); err != nil {
		return err
	}

	// Pull images
	if err := compose.Pull("docker-compose.prod.yml"); err != nil {
		return err
	}

	// Start services in detached mode
	if err := compose.Up("docker-compose.prod.yml", nil, false, false, true); err != nil {
		return err
	}

	return nil
}
