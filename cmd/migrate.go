package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/cjairm/rocketctl/internal/config"
	"github.com/cjairm/rocketctl/internal/migrate"
	"github.com/cjairm/rocketctl/internal/ssh"
	"github.com/spf13/cobra"
)

// migrateLogDir holds one log per migrate run. rocketctl keeps no other logs,
// so it sits next to rocket.yaml, named like .rocket-version.
const migrateLogDir = ".rocket-logs"

var (
	migrateApply bool
	migrateYes   bool
)

var migrateCmd = &cobra.Command{
	Use:   "migrate [service]",
	Short: "Run the app's release migrations in its running container",
	Long: `Runs the migration command the app declares with the image label
` + migrate.Label + `, inside the service's running container on the server.

Dry run by default: the command gets --dry-run. Pass --apply to make changes;
it asks for confirmation first unless --yes is given. An image without the
label has nothing to migrate. Output is streamed and saved to ` + migrateLogDir + `/.`,
	Args: cobra.MaximumNArgs(1),
	// A failed migration is not a usage mistake; don't bury its output.
	SilenceUsage: true,
	RunE:         runMigrate,
}

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.Flags().
		BoolVar(&migrateApply, "apply", false, "Apply migrations instead of a dry run")
	migrateCmd.Flags().
		BoolVarP(&migrateYes, "yes", "y", false, "Skip the --apply confirmation prompt")
}

func runMigrate(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	service, err := migrateService(cfg, args)
	if err != nil {
		return err
	}

	if cfg.IP == "" {
		return fmt.Errorf(
			"server IP is required to run migrations. Please run 'rocketctl init' to configure it",
		)
	}

	sshUser, err := resolveSSHUser(cfg)
	if err != nil {
		return err
	}
	host := fmt.Sprintf("%s@%s", sshUser, cfg.IP)

	fmt.Printf("📡 Connecting to %s...\n", host)
	client, err := ssh.Connect(cfg.IP, sshUser, cfg.SSHKeyPath, cfg.InsecureSkipHostKeyCheck)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer func() { _ = client.Close() }()

	return migrate.Run(client, migrate.Options{
		Host:    host,
		Project: cfg.Project,
		Service: service,
		Apply:   migrateApply,
		Yes:     migrateYes,
		LogDir:  migrateLogDir,
		In:      os.Stdin,
		Out:     os.Stdout,
		ErrOut:  os.Stderr,
		Now:     time.Now,
	})
}

// migrateService resolves which service to migrate. Unlike build and push, a
// name given in single-service mode is checked rather than ignored: running
// migrations for a service other than the one typed must not happen quietly.
func migrateService(cfg *config.Config, args []string) (string, error) {
	var service string
	switch {
	case len(args) > 0:
		service = args[0]
	case cfg.IsMonorepo():
		return "", fmt.Errorf(
			"service name is required for monorepo. Available services: %v",
			cfg.GetServices(),
		)
	default:
		service = cfg.Service
	}
	if err := cfg.ValidateService(service); err != nil {
		return "", err
	}
	return service, nil
}
