package cmd

import (
	"fmt"

	"github.com/cjairm/rocketctl/internal/config"
	"github.com/cjairm/rocketctl/internal/docker"
	"github.com/cjairm/rocketctl/internal/version"
	"github.com/spf13/cobra"
)

var (
	bumpType string
)

var buildCmd = &cobra.Command{
	Use:   "build [service]",
	Short: "Build a production Docker image",
	Long:  `Builds a production Docker image for a service, bumps the version, and updates .rocket-version.`,
	RunE:  runBuild,
}

func init() {
	rootCmd.AddCommand(buildCmd)
	buildCmd.Flags().StringVar(&bumpType, "bump", "patch", "Version bump type (major, minor, patch)")
}

func runBuild(cmd *cobra.Command, args []string) error {
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

	// Get version file path
	versionPath, err := cfg.GetVersionFilePath(service)
	if err != nil {
		return err
	}

	// Read current version
	currentVersion, err := version.Get(versionPath)
	if err != nil {
		return err
	}

	// Calculate new version
	newVersion, err := version.CalculateBump(currentVersion, bumpType)
	if err != nil {
		return err
	}

	fmt.Printf("Building %s: %s -> %s\n", service, currentVersion, newVersion)

	// Get service directory and Dockerfile path
	serviceDir, err := cfg.GetServiceDirectory(service)
	if err != nil {
		return err
	}

	dockerfilePath, err := cfg.GetDockerfilePath(service, true)
	if err != nil {
		return err
	}

	// Build the image with both local and registry tags
	imageName := cfg.GetImageName(service)
	registryImageName := fmt.Sprintf("%s/%s", cfg.Registry, imageName)

	buildOpts := docker.BuildOptions{
		ImageName:  imageName,
		Tag:        newVersion,
		Dockerfile: dockerfilePath,
		Context:    serviceDir,
		NoCache:    true,
	}

	if err := docker.Build(buildOpts); err != nil {
		return err
	}

	// Tag with registry
	if err := docker.Tag(imageName, newVersion, registryImageName, newVersion); err != nil {
		return err
	}

	// Update version file only on success
	if err := version.Set(versionPath, newVersion); err != nil {
		return err
	}

	fmt.Printf("✓ Successfully built and tagged %s:%s\n", registryImageName, newVersion)
	fmt.Printf("✓ Updated version to %s\n", newVersion)
	return nil
}
