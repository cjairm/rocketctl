package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cjairm/rocketctl/internal/backup"
	"github.com/cjairm/rocketctl/internal/config"
	"github.com/cjairm/rocketctl/internal/ssh"
	"github.com/spf13/cobra"
)

var (
	backupOut  string
	backupKeep int
)

var backupCmd = &cobra.Command{
	Use:   "backup [service]",
	Short: "Save the app's own backup from its running container",
	Long: `Runs the backup command the app declares with the image label
` + backup.Label + `, inside the service's running container on the server, and
streams its stdout straight into a local file. Nothing is written on the server.

The file is saved as <project>-<service>-<timestamp><suffix>, with the suffix
from the ` + backup.SuffixLabel + ` label (default ` + backup.DefaultSuffix + `), under
~/.rocketctl/backups/<project>/ unless --out is given. It is readable by you
alone. After a successful backup, only the newest --keep backups of that
service are kept. A failed or empty backup is discarded and never counted.
The command's messages are shown live and saved to ` + appLogDir + `/.`,
	Example: `  rocketctl backup api                 # save a backup of api
  rocketctl backup api --keep 10       # keep the newest 10
  rocketctl backup api --out /mnt/safe # save somewhere else
  rocketctl backup                     # single-service repo`,
	Args: cobra.MaximumNArgs(1),
	// A failed backup is not a usage mistake; don't bury its output.
	SilenceUsage: true,
	RunE:         runBackup,
}

func init() {
	rootCmd.AddCommand(backupCmd)
	backupCmd.Flags().
		StringVar(&backupOut, "out", "", "Folder to save the backup in (default ~/.rocketctl/backups/<project>)")
	backupCmd.Flags().
		IntVar(&backupKeep, "keep", 5, "How many backups of the service to keep, including this one")
}

func runBackup(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	service, err := runningService(cfg, args)
	if err != nil {
		return err
	}

	// Refuse before connecting, so a typo costs nothing.
	if backupKeep < 1 {
		return fmt.Errorf("--keep must be at least 1, got %d", backupKeep)
	}

	home, err := os.UserHomeDir()
	if err != nil && backupOut == "" {
		return fmt.Errorf("cannot find your home directory: %w. Pass --out <dir>", err)
	}
	dir := backupDir(home, cfg.Project, backupOut)

	if cfg.IP == "" {
		return fmt.Errorf(
			"server IP is required to take a backup. Please run 'rocketctl init' to configure it",
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

	return backup.Run(client, backup.Options{
		Host:    host,
		Project: cfg.Project,
		Service: service,
		Dir:     dir,
		Keep:    backupKeep,
		LogDir:  appLogDir,
		Out:     os.Stdout,
		ErrOut:  os.Stderr,
		Now:     time.Now,
	})
}

// backupDir is where backups are saved: out when given, otherwise a
// per-project folder under the home directory. Backups are real data, so the
// default is outside any repository.
func backupDir(home, project, out string) string {
	if out != "" {
		return out
	}
	return filepath.Join(home, ".rocketctl", "backups", project)
}
