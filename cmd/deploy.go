package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/cjairm/rocketctl/internal/compose"
	"github.com/cjairm/rocketctl/internal/config"
	"github.com/cjairm/rocketctl/internal/docker"
	"github.com/cjairm/rocketctl/internal/registry"
	"github.com/cjairm/rocketctl/internal/ssh"
	"github.com/cjairm/rocketctl/internal/version"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy services to remote server via SSH",
	Long: `Deploys to the server in rocket.yaml (ip, ssh_user, ssh_key_path) over SSH, into
~/apps/<project>. There is no dry run and no rollback.

It deploys each service's current .rocket-version, so build and push first.
Every compose service whose image is one rocketctl builds is pinned to that
version, matched by image rather than by name, so services that reuse a built
image follow it. Only the uploaded copy is changed, never your local file.

Steps: pin and upload docker-compose.prod.yml as docker-compose.yml; upload
caddy/Caddyfile when a domain is set; upload .env.example as .env only if the
server has no .env yet (edit the real values on the server); log in to ECR on
the server; docker compose pull; docker compose up -d.

With --clean, once the deploy succeeds, it frees space:
  server  docker container/image/volume/network/system prune (server-wide;
          volume prune is skipped on Docker older than 23)
  ECR     deletes this project's versions older than the previous one
  local   removes the same old versions of this project's images
The current and previous versions are always kept, so you can roll back.`,
	Example: `  rocketctl build api --bump patch --push
  rocketctl deploy

  # Deploy, then free disk space on the server, in ECR and locally
  rocketctl deploy --clean`,
	RunE: runDeploy,
}

var deployClean bool

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().BoolVar(
		&deployClean,
		"clean",
		false,
		"Free space after a successful deploy: prune the server, delete old versions from ECR and locally (keeps current and previous)",
	)
}

// remoteCleanupCommand is the server-wide prune run by deploy --clean. Every
// step takes -f: without it each one stops at a y/N prompt.
func remoteCleanupCommand(pruneVolumes bool) string {
	steps := []string{"docker container prune -f", "docker image prune -a -f"}
	if pruneVolumes {
		steps = append(steps, "docker volume prune -f")
	}
	steps = append(steps, "docker network prune -f", "docker system prune -a -f")
	return strings.Join(steps, " && ")
}

// volumePruneIsSafe reports whether the server's Docker is 23 or newer, where
// "docker volume prune" removes only anonymous volumes. Older versions also
// remove unused named volumes, which hold data. Unknown versions are unsafe.
func volumePruneIsSafe(serverVersion string) bool {
	major, _, _ := strings.Cut(strings.TrimSpace(serverVersion), ".")
	n, err := strconv.Atoi(major)
	return err == nil && n >= 23
}

// staleLocalImages picks the local images of a service, in both the bare and
// the registry-prefixed form, that are older than the version before current.
func staleLocalImages(cfg *config.Config, service, current string, images []string) []string {
	var stale []string
	for _, repository := range []string{cfg.GetImageName(service), cfg.GetImageRepository(service)} {
		var tags []string
		for _, image := range images {
			if tag, ok := strings.CutPrefix(image, repository+":"); ok {
				tags = append(tags, tag)
			}
		}
		for _, tag := range version.Stale(tags, current) {
			stale = append(stale, repository+":"+tag)
		}
	}
	return stale
}

// cleanAfterDeploy frees space once a deploy has succeeded: a Docker prune on
// the server, then ECR and local images older than the previous version.
func cleanAfterDeploy(client *ssh.Client, cfg *config.Config, serviceVersions map[string]string) error {
	fmt.Println("🧹 Cleaning up the server...")
	serverVersion, err := client.Exec("docker version --format '{{.Server.Version}}'")
	pruneVolumes := err == nil && volumePruneIsSafe(serverVersion)
	if !pruneVolumes {
		fmt.Println(
			"⚠️  Server Docker is older than 23 or unknown: skipping 'docker volume prune', which would also delete unused named volumes",
		)
	}
	if err := client.ExecInteractive(remoteCleanupCommand(pruneVolumes)); err != nil {
		return fmt.Errorf("server cleanup failed: %w", err)
	}

	fmt.Println("🧹 Deleting old images from ECR (keeping current and previous)...")
	for _, service := range cfg.GetServices() {
		repository := cfg.GetImageName(service)
		tags, err := registry.ListImageTags(repository, cfg.Region)
		if err != nil {
			return err
		}
		stale := version.Stale(tags, serviceVersions[service])
		if len(stale) == 0 {
			fmt.Printf("   %s: nothing to delete\n", repository)
			continue
		}
		if err := registry.DeleteImageTags(repository, cfg.Region, stale); err != nil {
			return err
		}
		fmt.Printf("   %s: deleted %s\n", repository, strings.Join(stale, ", "))
	}

	fmt.Println("🧹 Removing old local images (keeping current and previous)...")
	for _, service := range cfg.GetServices() {
		var images []string
		for _, repository := range []string{cfg.GetImageName(service), cfg.GetImageRepository(service)} {
			found, err := docker.ListImages(repository)
			if err != nil {
				return err
			}
			images = append(images, found...)
		}
		for _, image := range staleLocalImages(cfg, service, serviceVersions[service], images) {
			// A local container may still use it; that is not worth failing over.
			if err := docker.RemoveImage(image); err != nil {
				fmt.Printf("⚠️  %v\n", err)
				continue
			}
			fmt.Printf("   removed %s\n", image)
		}
	}
	return nil
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

// builtImageTags maps the image repository rocketctl builds for each service
// to the version being deployed, e.g. "reg/myapp_api" -> "1.4.0".
func builtImageTags(cfg *config.Config, serviceVersions map[string]string) map[string]string {
	tags := make(map[string]string, len(serviceVersions))
	for service, ver := range serviceVersions {
		tags[cfg.GetImageRepository(service)] = ver
	}
	return tags
}

// pinnedImageLines renders which compose services took each built image's
// tag, in rocket.yaml order, e.g. "myapp_api:1.4.0 → myapp-api, myapp-worker".
// A built image no service took is flagged rather than skipped, so a service
// the pinning could not reach is visible.
func pinnedImageLines(
	cfg *config.Config,
	serviceVersions map[string]string,
	updated map[string][]string,
) []string {
	var lines []string
	for _, service := range cfg.GetServices() {
		repository := cfg.GetImageRepository(service)
		target := strings.Join(updated[repository], ", ")
		if target == "" {
			target = fmt.Sprintf(
				"⚠️  not pinned: no service in docker-compose.prod.yml uses %s (YAML anchors and merge keys are not followed)",
				repository,
			)
		}
		lines = append(lines, fmt.Sprintf(
			"%s:%s → %s",
			cfg.GetImageName(service),
			serviceVersions[service],
			target,
		))
	}
	return lines
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

	// Pin the compose file before connecting, so a bad file fails without
	// touching the server. Every service reusing a built image gets its tag.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	localComposePath := filepath.Join(cwd, "docker-compose.prod.yml")
	composeInfo, err := os.Stat(localComposePath)
	if os.IsNotExist(err) {
		return fmt.Errorf(
			"docker-compose.prod.yml not found in project directory. Please create it first using 'rocketctl init'",
		)
	}
	if err != nil {
		return fmt.Errorf("failed to stat docker-compose.prod.yml: %w", err)
	}
	composeContent, err := os.ReadFile(localComposePath)
	if err != nil {
		return fmt.Errorf("failed to read docker-compose.prod.yml: %w", err)
	}
	pinnedCompose, updated, err := compose.PinImages(
		composeContent,
		builtImageTags(cfg, serviceVersions),
	)
	if err != nil {
		return err
	}
	fmt.Println("🏷  Pinning image tags:")
	for _, line := range pinnedImageLines(cfg, serviceVersions, updated) {
		fmt.Printf("   %s\n", line)
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

	// Upload the pinned docker-compose.prod.yml; the local file is untouched
	fmt.Println("📤 Uploading docker-compose.prod.yml...")
	remoteComposePath := fmt.Sprintf("%s/docker-compose.yml", remoteDir)
	if err := client.UploadContent(
		bytes.NewReader(pinnedCompose),
		remoteComposePath,
		composeInfo.Mode().Perm(),
	); err != nil {
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

	if deployClean {
		if err := cleanAfterDeploy(client, cfg, serviceVersions); err != nil {
			return fmt.Errorf("deployment succeeded, but --clean failed: %w", err)
		}
		fmt.Println("✅ Cleanup complete")
	}
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
