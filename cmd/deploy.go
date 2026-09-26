package cmd

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cjairm/rocketctl/internal/config"
	"github.com/cjairm/rocketctl/internal/ssh"
	"github.com/cjairm/rocketctl/internal/version"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy services to remote server via SSH",
	Long: `Uploads project files (docker-compose.prod.yml, caddy/Caddyfile, .env.example) to the remote server,
authenticates with ECR, pulls latest images, and restarts services.

The deploy command uses files from your project directory, not templates. Ensure your project has:
- docker-compose.prod.yml (required)
- caddy/Caddyfile (if using domain/reverse proxy)
- .env.example (for initial .env creation on server)`,
	RunE: runDeploy,
}

func init() {
	rootCmd.AddCommand(deployCmd)
}

// versionEnvAssignments renders the environment prefix that pins each service's
// image tag for the remote compose commands, e.g.
// "API_VERSION='0.1.4' WEB_VERSION='2.0.1' ".
//
// Without it the generated compose file falls back to ":latest", a tag that
// build and push never create. Services are sorted so the command is stable
// and readable in the deploy output.
func versionEnvAssignments(serviceVersions map[string]string) string {
	services := make([]string, 0, len(serviceVersions))
	for service := range serviceVersions {
		services = append(services, service)
	}
	sort.Strings(services)

	var b strings.Builder
	for _, service := range services {
		fmt.Fprintf(
			&b,
			"%s=%s ",
			config.EnvVersionKey(service),
			ssh.ShellQuote(serviceVersions[service]),
		)
	}
	return b.String()
}

// collectServiceVersions reads the local .rocket-version for every service so
// the remote pulls exactly what the last build produced.
func collectServiceVersions(cfg *config.Config) (map[string]string, error) {
	versions := make(map[string]string, len(cfg.GetServices()))
	for _, service := range cfg.GetServices() {
		versionPath, err := cfg.GetVersionFilePath(service)
		if err != nil {
			return nil, err
		}
		ver, err := version.Get(versionPath)
		if err != nil {
			return nil, err
		}
		versions[service] = ver
	}
	return versions, nil
}

// resolveSSHUser returns ssh_user from rocket.yaml, falling back to the local
// user the way ssh itself does.
func resolveSSHUser(cfg *config.Config) (string, error) {
	if cfg.SSHUser != "" {
		return cfg.SSHUser, nil
	}
	currentUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}
	return currentUser.Username, nil
}

func runDeploy(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.IP == "" {
		return fmt.Errorf(
			"server IP is required for deployment. Please run 'rocketctl init' to configure it",
		)
	}
	serviceVersions, err := collectServiceVersions(cfg)
	if err != nil {
		return err
	}
	versionEnv := versionEnvAssignments(serviceVersions)
	fmt.Println("📌 Deploying versions:")
	for _, service := range cfg.GetServices() {
		fmt.Printf("   %s %s\n", service, serviceVersions[service])
	}

	sshUser, err := resolveSSHUser(cfg)
	if err != nil {
		return err
	}
	fmt.Printf("🚀 Deploying to %s@%s...\n", sshUser, cfg.IP)

	// Connect to server via SSH
	fmt.Println("📡 Connecting to server...")
	client, err := ssh.Connect(cfg.IP, sshUser, cfg.SSHKeyPath, cfg.InsecureSkipHostKeyCheck)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer func() { _ = client.Close() }()

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Create remote directory structure
	// The "~" stays outside the quotes so the remote shell still expands it.
	remoteDir := "~/apps/" + ssh.ShellQuote(cfg.Project)
	// Unquoted twin, only for the copy-pasteable hints printed at the end.
	// Safe to show because Validate restricts Project to [A-Za-z0-9._-].
	remoteDirDisplay := fmt.Sprintf("~/apps/%s", cfg.Project)
	fmt.Printf("📁 Creating directory structure: %s\n", remoteDir)
	if err := client.MkdirAll(remoteDir); err != nil {
		return err
	}

	// Upload docker-compose.prod.yml from project directory
	localComposePath := filepath.Join(cwd, "docker-compose.prod.yml")
	if _, err := os.Stat(localComposePath); os.IsNotExist(err) {
		return fmt.Errorf(
			"docker-compose.prod.yml not found in project directory. Please create it first using 'rocketctl init'",
		)
	}
	fmt.Println("📤 Uploading docker-compose.prod.yml...")
	remoteComposePath := fmt.Sprintf("%s/docker-compose.yml", remoteDir)
	if err := client.UploadFile(localComposePath, remoteComposePath); err != nil {
		return fmt.Errorf("failed to upload docker-compose.prod.yml: %w", err)
	}

	// Upload Caddyfile if domain is configured and file exists
	if cfg.Domain != "" {
		localCaddyPath := filepath.Join(cwd, "caddy", "Caddyfile")
		if _, err := os.Stat(localCaddyPath); err == nil {
			fmt.Println("📤 Uploading Caddyfile...")
			// Create caddy directory on remote
			remoteCaddyDir := fmt.Sprintf("%s/caddy", remoteDir)
			if err := client.MkdirAll(remoteCaddyDir); err != nil {
				return err
			}
			remoteCaddyPath := fmt.Sprintf("%s/caddy/Caddyfile", remoteDir)
			if err := client.UploadFile(localCaddyPath, remoteCaddyPath); err != nil {
				return fmt.Errorf("failed to upload Caddyfile: %w", err)
			}
		} else {
			fmt.Println("⚠️  caddy/Caddyfile not found in project directory, skipping...")
		}
	}

	// Handle .env files based on monorepo structure
	// Helper function to handle .env upload for a given path
	uploadEnvFile := func(service string) error {
		// Determine paths based on monorepo vs non-monorepo
		var localEnvExamplePath, remoteDotEnvPath, serviceDir string
		if cfg.IsMonorepo() {
			serviceDir = fmt.Sprintf("%s/%s", remoteDir, service)
			remoteDotEnvPath = fmt.Sprintf("%s/.env", serviceDir)
			localEnvExamplePath = filepath.Join(cwd, service, ".env.example")
		} else {
			serviceDir = remoteDir
			remoteDotEnvPath = fmt.Sprintf("%s/.env", remoteDir)
			localEnvExamplePath = filepath.Join(cwd, ".env.example")
		}

		// Check if .env exists on remote
		exists, err := client.FileExists(remoteDotEnvPath)
		if err != nil {
			if service != "" {
				return fmt.Errorf("failed to check if %s/.env exists: %w", service, err)
			}
			return fmt.Errorf("failed to check if .env exists: %w", err)
		}

		if !exists {
			if _, err := os.Stat(localEnvExamplePath); err == nil {
				// Create service directory if needed
				if err := client.MkdirAll(serviceDir); err != nil {
					return err
				}
				if service != "" {
					fmt.Printf("📤 Uploading %s/.env from %s/.env.example...\n", service, service)
				} else {
					fmt.Println("📤 Uploading .env from .env.example...")
				}
				// 0600: this file holds production secrets.
				if err := client.UploadFileMode(
					localEnvExamplePath,
					remoteDotEnvPath,
					0o600,
				); err != nil {
					if service != "" {
						return fmt.Errorf("failed to upload %s/.env: %w", service, err)
					}
					return fmt.Errorf("failed to upload .env: %w", err)
				}
				if service != "" {
					fmt.Printf(
						"⚠️  Remember to edit %s/.env on the server with actual values!\n",
						service,
					)
				} else {
					fmt.Println("⚠️  Remember to edit .env on the server with actual values!")
				}
			} else {
				if service != "" {
					fmt.Printf(
						"⚠️  %s/.env.example not found and %s/.env doesn't exist on server\n",
						service,
						service,
					)
				} else {
					fmt.Println(
						"⚠️  .env.example not found in project directory and .env doesn't exist on server",
					)
					fmt.Println("⚠️  You may need to manually create .env on the server")
				}
			}
		} else {
			if service != "" {
				fmt.Printf("✅ %s/.env already exists on server, skipping...\n", service)
			} else {
				fmt.Println("✅ .env already exists on server, skipping...")
			}
		}
		return nil
	}

	// Process .env files
	if cfg.IsMonorepo() {
		fmt.Println("📝 Handling .env files for monorepo services...")
		for _, service := range cfg.Services {
			if err := uploadEnvFile(service); err != nil {
				return err
			}
		}
	} else {
		if err := uploadEnvFile(""); err != nil {
			return err
		}
	}

	// Authenticate with ECR
	fmt.Println("🔐 Authenticating with ECR...")
	loginCmd := fmt.Sprintf(
		"aws ecr get-login-password --region %s | docker login --username AWS --password-stdin %s",
		cfg.Region,
		cfg.Registry,
	)
	if err := client.ExecInteractive(fmt.Sprintf("cd %s && %s", remoteDir, loginCmd)); err != nil {
		return fmt.Errorf("ECR authentication failed: %w", err)
	}

	// Pull latest images
	fmt.Println("📥 Pulling latest images...")
	pullCmd := "docker compose pull"
	if err := client.ExecInteractive(
		fmt.Sprintf("cd %s && %s%s", remoteDir, versionEnv, pullCmd),
	); err != nil {
		return fmt.Errorf("failed to pull images: %w", err)
	}

	// Start services
	fmt.Println("🚀 Starting services...")
	upCmd := "docker compose up -d"
	if err := client.ExecInteractive(
		fmt.Sprintf("cd %s && %s%s", remoteDir, versionEnv, upCmd),
	); err != nil {
		return fmt.Errorf("failed to start services: %w", err)
	}

	fmt.Println("✅ Deployment successful!")
	fmt.Printf(
		"\n📊 To view logs: ssh %s@%s 'cd %s && docker compose logs -f'\n",
		sshUser,
		cfg.IP,
		remoteDirDisplay,
	)
	fmt.Printf(
		"📊 To view status: ssh %s@%s 'cd %s && docker compose ps'\n",
		sshUser,
		cfg.IP,
		remoteDirDisplay,
	)

	return nil
}
