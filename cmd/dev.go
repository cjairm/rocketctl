package cmd

import (
	"fmt"

	"github.com/cjairm/rocketctl/internal/compose"
	"github.com/cjairm/rocketctl/internal/config"
	"github.com/spf13/cobra"
)

var (
	buildFlag   bool
	noCacheFlag bool
)

var devCmd = &cobra.Command{
	Use:   "dev [service]",
	Short: "Start the development environment",
	Long:  `Starts the development environment using docker compose.`,
	RunE:  runDev,
}

func init() {
	rootCmd.AddCommand(devCmd)
	devCmd.Flags().BoolVar(&buildFlag, "build", false, "Rebuild images before starting")
	devCmd.Flags().BoolVar(&noCacheFlag, "no-cache", false, "Rebuild without cache (requires --build)")
}

func runDev(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	var services []string
	if len(args) > 0 {
		service := args[0]
		if err := cfg.ValidateService(service); err != nil {
			return err
		}
		// Compose service name format: <project>-<service>
		services = []string{fmt.Sprintf("%s-%s", cfg.Project, service)}
	}

	if noCacheFlag && !buildFlag {
		return fmt.Errorf("--no-cache requires --build")
	}

	return compose.Up("docker-compose.yml", services, buildFlag, noCacheFlag, false)
}
