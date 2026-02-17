package cmd

import (
	"fmt"

	"github.com/cjairm/rocketctl/internal/config"
	"github.com/cjairm/rocketctl/internal/docker"
	"github.com/cjairm/rocketctl/internal/version"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test [service]",
	Short: "Test a production build locally",
	Long:  `Builds and runs a production image locally for testing without bumping the version.`,
	RunE:  runTest,
}

func init() {
	rootCmd.AddCommand(testCmd)
}

func runTest(cmd *cobra.Command, args []string) error {
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

	// Read current version (no bump)
	currentVersion, err := version.Get(versionPath)
	if err != nil {
		return err
	}

	fmt.Printf("Testing %s at version %s\n", service, currentVersion)

	// Get service directory and Dockerfile path
	serviceDir, err := cfg.GetServiceDirectory(service)
	if err != nil {
		return err
	}

	dockerfilePath, err := cfg.GetDockerfilePath(service, true)
	if err != nil {
		return err
	}

	// Load .env.production for build args if it exists
	envProductionPath, err := cfg.GetEnvProductionPath(service)
	if err != nil {
		return err
	}

	buildArgs, err := docker.LoadEnvFile(envProductionPath)
	if err != nil {
		return err
	}

	// Build the image
	imageName := cfg.GetImageName(service)

	buildOpts := docker.BuildOptions{
		ImageName:  imageName,
		Tag:        currentVersion,
		Dockerfile: dockerfilePath,
		Context:    serviceDir,
		BuildArgs:  buildArgs,
		NoCache:    false,
	}

	if err := docker.Build(buildOpts); err != nil {
		return err
	}

	// Run the container in detached mode
	containerID, err := docker.Run(imageName, currentVersion, envProductionPath, true)
	if err != nil {
		return err
	}

	fmt.Printf("\n✓ Container is running with ID: %s\n", containerID)
	fmt.Println("To view logs: docker logs -f", containerID)
	fmt.Println("To stop: docker stop", containerID)

	return nil
}
