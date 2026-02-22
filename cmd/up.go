package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cjairm/rocketctl/internal/compose"
	"github.com/cjairm/rocketctl/internal/config"
	"github.com/cjairm/rocketctl/internal/docker"
	"github.com/cjairm/rocketctl/internal/version"
	"github.com/spf13/cobra"
)

var (
	upProdFlag    bool
	upBuildFlag   bool
	upNoCacheFlag bool
)

var upCmd = &cobra.Command{
	Use:   "up [service]",
	Short: "Start services with docker compose",
	Long:  `Starts services using docker compose. Use --prod to build and run the production stack for E2E testing. Optionally specify a service to build only that service (with --prod), but the entire stack will still be started.`,
	RunE:  runUp,
}

func init() {
	upCmd.Flags().BoolVar(&upProdFlag, "prod", false, "Use production configuration (docker-compose.prod.yml)")
	upCmd.Flags().BoolVar(&upBuildFlag, "build", false, "Rebuild images before starting (dev mode only)")
	upCmd.Flags().BoolVar(&upNoCacheFlag, "no-cache", false, "Rebuild without cache (requires --build, dev mode only)")
	rootCmd.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Determine which compose file to use
	composeFile := "docker-compose.yml"
	if upProdFlag {
		composeFile = "docker-compose.prod.yml"
		// Check if docker-compose.prod.yml exists
		if _, err := os.Stat(composeFile); os.IsNotExist(err) {
			return fmt.Errorf("docker-compose.prod.yml not found. Run 'rocketctl init' first")
		}
	}

	// If not using --prod flag, just run regular dev compose up
	if !upProdFlag {
		// Validate --no-cache requires --build
		if upNoCacheFlag && !upBuildFlag {
			return fmt.Errorf("--no-cache requires --build")
		}

		fmt.Printf("Starting development environment with %s...\n", composeFile)
		return compose.Up(composeFile, args, upBuildFlag, upNoCacheFlag, false)
	}

	// Determine which services to build
	// If service is specified, build only that one; otherwise build all
	var servicesToBuild []string
	if len(args) > 0 {
		service := args[0]
		if err := cfg.ValidateService(service); err != nil {
			return err
		}
		servicesToBuild = []string{service}
		fmt.Printf("Building service: %s\n", service)
	} else {
		servicesToBuild = cfg.GetServices()
		fmt.Printf("Building all services: %v\n", servicesToBuild)
	}

	// Build and tag each service
	for _, service := range servicesToBuild {
		versionPath, err := cfg.GetVersionFilePath(service)
		if err != nil {
			return err
		}
		currentVersion, err := version.Get(versionPath)
		if err != nil {
			return err
		}

		fmt.Printf("\n[%s] Building version %s...\n", service, currentVersion)

		serviceDir, err := cfg.GetServiceDirectory(service)
		if err != nil {
			return err
		}
		dockerfilePath, err := cfg.GetDockerfilePath(service, true)
		if err != nil {
			return err
		}

		// Build the image locally
		imageName := cfg.GetImageName(service)
		buildOpts := docker.BuildOptions{
			ImageName:  imageName,
			Tag:        currentVersion,
			Dockerfile: dockerfilePath,
			Context:    serviceDir,
			NoCache:    false,
		}
		if err := docker.Build(buildOpts); err != nil {
			return err
		}

		// Tag the image to match docker-compose.prod.yml expectations
		// Compose expects: {registry}/{project}_{service}:{version}
		registryImage := fmt.Sprintf("%s/%s", cfg.Registry, imageName)
		if err := docker.Tag(imageName, currentVersion, registryImage, currentVersion); err != nil {
			return err
		}

		// Set environment variable for the service version
		// docker-compose.prod.yml uses ${SERVICE_VERSION:-latest}
		envKey := fmt.Sprintf("%s_VERSION", strings.ToUpper(service))
		os.Setenv(envKey, currentVersion)
	}

	// Start the entire stack using docker compose
	fmt.Printf("\n🚀 Starting entire production stack with docker-compose.prod.yml...\n")
	if err := compose.Up("docker-compose.prod.yml", nil, false, false, true); err != nil {
		return err
	}

	fmt.Printf("\n✓ Production stack is running\n")
	fmt.Printf("Services: %v\n", cfg.GetServices())
	fmt.Println("\nUseful commands:")
	fmt.Println("  View logs (all):     rocketctl logs --prod -f")
	fmt.Println("  View logs (service): rocketctl logs --prod <service> -f")
	fmt.Println("  List containers:     rocketctl ps")
	fmt.Println("  Stop all:            rocketctl down --prod")
	return nil
}
