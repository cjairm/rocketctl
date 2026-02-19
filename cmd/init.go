package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cjairm/rocketctl/internal/config"
	"github.com/cjairm/rocketctl/internal/templates"
	"github.com/cjairm/rocketctl/internal/version"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a project for use with RocketCTL",
	Long:  `Creates rocket.yaml, .rocket-version files, and template files for a new project.`,
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check if rocket.yaml already exists
	if _, err := os.Stat("rocket.yaml"); err == nil {
		return fmt.Errorf("rocket.yaml already exists. Remove it first if you want to reinitialize")
	}
	reader := bufio.NewReader(os.Stdin)

	// Get current directory name as default project name
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defaultProjectName := filepath.Base(cwd)

	// Prompt for project name
	fmt.Printf("Project name [%s]: ", defaultProjectName)
	projectName, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read \"Project Name\" response: %w", err)
	}
	projectName = strings.TrimSpace(projectName)
	if projectName == "" {
		projectName = defaultProjectName
	}

	// Prompt for registry URL
	fmt.Print("Container registry URL: ")
	registry, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read \"Container registry URL\" response: %w", err)
	}
	registry = strings.TrimSpace(registry)
	if registry == "" {
		return fmt.Errorf("registry URL is required")
	}

	// Prompt for AWS region
	fmt.Print("AWS region [us-east-2]: ")
	region, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read \"AWS region\" response: %w", err)
	}
	region = strings.TrimSpace(region)
	if region == "" {
		region = "us-east-2"
	}

	// Prompt for domain (optional)
	fmt.Print("Domain (optional, for Caddy configuration): ")
	domain, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read \"Domain\" response: %w", err)
	}
	domain = strings.TrimSpace(domain)

	// Prompt for repo type
	fmt.Print("Is this a monorepo? (y/n) [n]: ")
	repoType, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read \"Is this a monorepo?\" response: %w", err)
	}
	repoType = strings.TrimSpace(strings.ToLower(repoType))
	isMonorepo := repoType == "y" || repoType == "yes"

	var cfg config.Config
	cfg.Project = projectName
	cfg.Registry = registry
	cfg.Region = region
	cfg.Domain = domain

	if isMonorepo {
		// Prompt for service names
		fmt.Print("Service names (comma-separated): ")
		servicesInput, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read \"Service names\" response: %w", err)
		}
		servicesInput = strings.TrimSpace(servicesInput)
		if servicesInput == "" {
			return fmt.Errorf("at least one service name is required")
		}
		services := strings.Split(servicesInput, ",")
		for i, s := range services {
			services[i] = strings.TrimSpace(s)
		}
		cfg.Services = services
	} else {
		// Prompt for service name
		fmt.Print("Service name: ")
		serviceName, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read \"Service name\" response: %w", err)
		}
		serviceName = strings.TrimSpace(serviceName)
		if serviceName == "" {
			return fmt.Errorf("service name is required")
		}
		cfg.Service = serviceName
	}

	// Save rocket.yaml
	if err := cfg.Save("rocket.yaml"); err != nil {
		return err
	}
	fmt.Println("✓ Created rocket.yaml")

	// Create .rocket-version files
	for _, service := range cfg.GetServices() {
		versionPath, err := cfg.GetVersionFilePath(service)
		if err != nil {
			return err
		}
		// Create service directory if it doesn't exist (monorepo mode)
		if cfg.IsMonorepo() {
			serviceDir := filepath.Dir(versionPath)
			if err := os.MkdirAll(serviceDir, 0755); err != nil {
				return fmt.Errorf("failed to create service directory %s: %w", serviceDir, err)
			}
		}
		if err := version.Set(versionPath, "0.1.0"); err != nil {
			return err
		}
		fmt.Printf("✓ Created %s\n", versionPath)
	}

	// Generate docker-compose.prod.yml
	templateData := templates.TemplateData{
		Project:    cfg.Project,
		Services:   cfg.GetServices(),
		Registry:   cfg.Registry,
		Region:     cfg.Region,
		Domain:     cfg.Domain,
		IsMonorepo: cfg.IsMonorepo(),
	}
	if err := templates.GenerateDockerComposeProd(templateData, "docker-compose.prod.yml"); err != nil {
		return err
	}

	// Generate Caddyfile if domain was provided
	if cfg.Domain != "" {
		fmt.Print("Email for Caddy HTTPS certificates: ")
		email, _ := reader.ReadString('\n')
		email = strings.TrimSpace(email)
		templateData.Email = email
		if err := templates.GenerateCaddyfile(templateData, "caddy/Caddyfile"); err != nil {
			return err
		}
	}

	fmt.Println("\n✓ Initialization complete!")
	fmt.Println("\nNext steps:")
	fmt.Println("1. Create Dockerfile and Dockerfile.production for each service")
	fmt.Println("2. Create docker-compose.yml for your development environment")
	fmt.Println("3. Create .env.production on the server with the required secrets")
	fmt.Println("4. Customize docker-compose.prod.yml as needed")
	if cfg.Domain != "" {
		fmt.Println("5. Customize caddy/Caddyfile as needed")
	}
	return nil
}
