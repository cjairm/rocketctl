package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

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

	InsecureSkipHostKeyCheck bool `yaml:"insecure_skip_host_key_check,omitempty"` // Opt out of known_hosts verification
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

	if err := validateName("project", c.Project); err != nil {
		return err
	}
	if err := validateHost("registry", c.Registry); err != nil {
		return err
	}
	if err := validateName("region", c.Region); err != nil {
		return err
	}

	// Two services whose names differ only by '-', '.' or '_' would share one
	// compose variable, silently pinning both to whichever version is written
	// last. Reject that here rather than deploying the wrong image.
	seen := make(map[string]string, len(c.Services))
	for _, service := range c.GetServices() {
		if err := validateName("service", service); err != nil {
			return err
		}
		key := EnvVersionKey(service)
		if other, ok := seen[key]; ok {
			return fmt.Errorf(
				"services %q and %q both map to the compose variable %s in rocket.yaml: rename one so their versions can be pinned independently",
				other,
				service,
				key,
			)
		}
		seen[key] = service
	}

	return nil
}

// namePattern matches names that are safe everywhere RocketCTL puts them: a
// Docker image component, a remote shell path segment and a compose variable.
var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// hostPattern matches a registry host, optionally with a port. Registry and
// region are interpolated unquoted into the remote ECR login command, so they
// need the same treatment as project and service names.
var hostPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]*(:[0-9]+)?$`)

// validateName rejects names that would be unsafe once interpolated into a
// remote shell command or an image reference.
func validateName(kind, name string) error {
	if name == "" {
		return fmt.Errorf("%s name cannot be empty in rocket.yaml", kind)
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf(
			"invalid %s name %q in rocket.yaml: use only letters, digits, '.', '_' and '-', starting with a letter or digit",
			kind,
			name,
		)
	}
	return nil
}

// validateHost rejects a registry or region that would be unsafe once
// interpolated into the remote "aws ecr get-login-password ... | docker login"
// command.
func validateHost(kind, value string) error {
	if !hostPattern.MatchString(value) {
		return fmt.Errorf(
			"invalid %s %q in rocket.yaml: use only letters, digits, '.', '-' and an optional ':port'",
			kind,
			value,
		)
	}
	return nil
}

// EnvVersionKey returns the compose interpolation variable carrying a service's
// version, e.g. "my-api" -> "MY_API_VERSION". Anything that is not a letter or
// digit becomes an underscore because compose variables must be shell
// identifiers - "MY-API_VERSION" silently renders an invalid image reference.
func EnvVersionKey(service string) string {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, service)
	return strings.ToUpper(safe) + "_VERSION"
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

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write rocket.yaml: %w", err)
	}

	return nil
}
