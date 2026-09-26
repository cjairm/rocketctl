package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cjairm/rocketctl/internal/backup"
	"github.com/cjairm/rocketctl/internal/config"
	"github.com/cjairm/rocketctl/internal/container"
	"github.com/cjairm/rocketctl/internal/version"
	"github.com/spf13/cobra"
)

var (
	backupOut     string
	backupKeep    int
	backupEnvFile string
)

var backupCmd = &cobra.Command{
	Use:   "backup [service]",
	Short: "Save the app's own backup from its running container",
	Long: `Runs the backup command the app declares with the image label
` + backup.Label + ` on this machine, in a throwaway container of the service's
image at its .rocket-version (built by 'rocketctl build'), with the settings in
<service dir>/.env.backup. Nothing runs on the server: the app's data store
must accept connections from this machine. The settings file must be readable
by you alone (chmod 600) and not tracked by git; rocketctl never reads it.

The file is saved as <project>-<service>-<timestamp><suffix>, with the suffix
from the ` + backup.SuffixLabel + ` label (default ` + backup.DefaultSuffix + `), under
~/.rocketctl/backups/<project>/ unless --out is given. It is readable by you
alone. After a successful backup, only the newest --keep backups of that
service are kept. A failed or empty backup is discarded and never counted.
The command's messages are shown live and saved to ` + appLogDir + `/.`,
	Example: `  rocketctl backup api                 # save a backup of api
  rocketctl backup api --keep 10       # keep the newest 10
  rocketctl backup api --out /mnt/safe # save somewhere else
  rocketctl backup api --env-file ~/secrets/api.env
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
	backupCmd.Flags().
		StringVar(&backupEnvFile, "env-file", "", "Settings to run the backup with (default <service dir>/.env.backup)")
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

	// Refuse before doing anything, so a typo costs nothing.
	if backupKeep < 1 {
		return fmt.Errorf("--keep must be at least 1, got %d", backupKeep)
	}

	home, err := os.UserHomeDir()
	if err != nil && backupOut == "" {
		return fmt.Errorf("cannot find your home directory: %w. Pass --out <dir>", err)
	}

	envFile := backupEnvFile
	if envFile == "" {
		if envFile, err = cfg.GetBackupEnvPath(service); err != nil {
			return err
		}
	}

	// The image the service's .rocket-version names, as 'rocketctl build'
	// tagged it on this machine.
	versionFile, err := cfg.GetVersionFilePath(service)
	if err != nil {
		return err
	}
	currentVersion, err := version.Get(versionFile)
	if err != nil {
		return err
	}

	return backup.Run(container.Local{}, backup.Options{
		Image:   cfg.GetFullImageName(service, currentVersion),
		EnvFile: envFile,
		Project: cfg.Project,
		Service: service,
		Dir:     backupDir(home, cfg.Project, backupOut),
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
