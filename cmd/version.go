package cmd

import (
	"fmt"

	"github.com/cjairm/rocketctl/internal/config"
	"github.com/cjairm/rocketctl/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version [service]",
	Short: "Show version for a service",
	Long:  `Shows the current version of a service or all services.`,
	RunE:  runVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func runVersion(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// If service specified, show only that service
	if len(args) > 0 {
		service := args[0]
		if err := cfg.ValidateService(service); err != nil {
			return err
		}

		versionPath, err := cfg.GetVersionFilePath(service)
		if err != nil {
			return err
		}

		ver, err := version.Get(versionPath)
		if err != nil {
			return err
		}

		fmt.Printf("%s: %s\n", service, ver)
		return nil
	}

	// Show all services
	for _, service := range cfg.GetServices() {
		versionPath, err := cfg.GetVersionFilePath(service)
		if err != nil {
			return err
		}

		ver, err := version.Get(versionPath)
		if err != nil {
			return err
		}

		fmt.Printf("%s: %s\n", service, ver)
	}

	return nil
}
