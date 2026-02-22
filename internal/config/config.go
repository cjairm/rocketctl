package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

// Config represents the rocket.yaml configuration file
type Config struct {
	Project    string   `yaml:"project"`
	Service    string   `yaml:"service,omitempty"`  // For single-service repos
	Services   []string `yaml:"services,omitempty"` // For monorepos
	Registry   string   `yaml:"registry"`
	Region     string   `yaml:"region"`
	Domain     string   `yaml:"domain,omitempty"`
	IP         string   `yaml:"ip,omitempty"`           // Server IP for SSH deployment
	SSHUser    string   `yaml:"ssh_user,omitempty"`     // SSH user (defaults to current user)
	SSHKeyPath string   `yaml:"ssh_key_path,omitempty"` // Custom SSH key path (e.g., ~/my-key.pem)
}

// Load reads and parses the rocket.yaml file from the current directory
func Load() (*Config, error) {
	data, err := os.ReadFile("rocket.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read rocket.yaml: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse rocket.yaml: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate ensures the configuration is valid
func (c *Config) Validate() error {
	if c.Project == "" {
		return fmt.Errorf("project field is required in rocket.yaml")
	}
	if c.Registry == "" {
		return fmt.Errorf("registry field is required in rocket.yaml")
	}
	if c.Region == "" {
		return fmt.Errorf("region field is required in rocket.yaml")
	}

	// Both service and services cannot be present
	if c.Service != "" && len(c.Services) > 0 {
		return fmt.Errorf("cannot have both 'service' and 'services' fields in rocket.yaml")
	}

	// At least one must be present
	if c.Service == "" && len(c.Services) == 0 {
		return fmt.Errorf(
			"either 'service' (single-service) or 'services' (monorepo) field is required in rocket.yaml",
		)
	}

	return nil
}

// IsMonorepo returns true if this is a monorepo configuration
func (c *Config) IsMonorepo() bool {
	return len(c.Services) > 0
}

// GetServices returns all services as a slice
func (c *Config) GetServices() []string {
	if c.IsMonorepo() {
		return c.Services
	}
	return []string{c.Service}
}

// ValidateService checks if a service name is valid for this configuration
func (c *Config) ValidateService(service string) error {
	services := c.GetServices()
	if !slices.Contains(services, service) {
		return fmt.Errorf(
			"service '%s' not found in configuration. Available services: %v",
			service,
			services,
		)
	}
	return nil
}

// GetServiceDirectory returns the directory path for a given service
func (c *Config) GetServiceDirectory(service string) (string, error) {
	if err := c.ValidateService(service); err != nil {
		return "", err
	}
	if c.IsMonorepo() {
		// In monorepo mode, service directory is a subfolder
		return filepath.Join(".", service), nil
	}
	// In single-service mode, service directory is the project root
	return ".", nil
}

// GetVersionFilePath returns the path to the .rocket-version file for a service
func (c *Config) GetVersionFilePath(service string) (string, error) {
	serviceDir, err := c.GetServiceDirectory(service)
	if err != nil {
		return "", err
	}
	return filepath.Join(serviceDir, ".rocket-version"), nil
}

// GetImageName returns the full image name (without registry) for a service
// Format: <project>_<service>
func (c *Config) GetImageName(service string) string {
	return fmt.Sprintf("%s_%s", c.Project, service)
}

// GetFullImageName returns the full image name including registry and version
// Format: <registry>/<project>_<service>:<version>
func (c *Config) GetFullImageName(service, version string) string {
	return fmt.Sprintf("%s/%s:%s", c.Registry, c.GetImageName(service), version)
}

// GetEnvProductionPath returns the path to .env.production for a service
func (c *Config) GetEnvProductionPath(service string) (string, error) {
	serviceDir, err := c.GetServiceDirectory(service)
	if err != nil {
		return "", err
	}
	return filepath.Join(serviceDir, ".env.production"), nil
}

// GetDockerfilePath returns the path to a Dockerfile for a service
func (c *Config) GetDockerfilePath(service string, production bool) (string, error) {
	serviceDir, err := c.GetServiceDirectory(service)
	if err != nil {
		return "", err
	}
	if production {
		return filepath.Join(serviceDir, "Dockerfile.production"), nil
	}
	return filepath.Join(serviceDir, "Dockerfile"), nil
}

// Save writes the configuration to rocket.yaml
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write rocket.yaml: %w", err)
	}

	return nil
}
