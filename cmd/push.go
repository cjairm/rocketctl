package cmd

import (
	"fmt"

	"github.com/cjairm/rocketctl/internal/config"
	"github.com/cjairm/rocketctl/internal/docker"
	"github.com/cjairm/rocketctl/internal/registry"
	"github.com/cjairm/rocketctl/internal/version"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push [service]",
	Short: "Push a built image to the container registry",
	Long:  `Authenticates with the container registry and pushes the built image.`,
	RunE:  runPush,
}

func init() {
	rootCmd.AddCommand(pushCmd)
}

func runPush(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Determine service name
	var service string
	if cfg.IsMonorepo() {
		if len(args) == 0 {
			return fmt.Errorf("service name is required for monorepo. Available services: %v", cfg.GetServices())
		}
		service = args[0]
	} else {
		service = cfg.Service
	}

	// Validate service
	if err := cfg.ValidateService(service); err != nil {
		return err
	}

	// Get current version
	versionPath, err := cfg.GetVersionFilePath(service)
	if err != nil {
		return err
	}

	currentVersion, err := version.Get(versionPath)
	if err != nil {
		return err
	}

	// Authenticate with ECR
	if err := registry.LoginECR(cfg.Registry, cfg.Region); err != nil {
		return err
	}

	// Push the image
	fullImageName := cfg.GetFullImageName(service, currentVersion)
	if err := docker.Push(fmt.Sprintf("%s/%s", cfg.Registry, cfg.GetImageName(service)), currentVersion); err != nil {
		return err
	}

	fmt.Printf("✓ Successfully pushed %s\n", fullImageName)
	return nil
}
