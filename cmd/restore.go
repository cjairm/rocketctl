package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/cjairm/rocketctl/internal/config"
	"github.com/cjairm/rocketctl/internal/container"
	"github.com/cjairm/rocketctl/internal/restore"
	"github.com/spf13/cobra"
)

// devComposeFile is the dev stack 'rocketctl up' starts.
const devComposeFile = "docker-compose.yml"

var restoreYes bool

var restoreCmd = &cobra.Command{
	Use:   "restore [service] [file]",
	Short: "Load a backup into the local dev stack",
	Long: `Loads a backup into the local dev stack started by 'rocketctl up' - never
the server. Runs the restore command the app declares with the image label
` + restore.Label + ` in the service's running dev container, with the backup on
stdin. The label goes in the dev Dockerfile, since that is the image the dev
container runs.

Without a file, restores the newest backup of the service from
~/.rocketctl/backups/<project>/. Asks for confirmation first unless --yes is
given. An image without the label has nothing to restore. Output is streamed
and saved to ` + appLogDir + `/.`,
	Example: `  rocketctl restore api                           # newest backup of api
  rocketctl restore api ~/backups/myapp-api-20260924-010101.tar.gz
  rocketctl restore api --yes                     # no prompt
  rocketctl restore                               # single-service repo`,
	Args: cobra.MaximumNArgs(2),
	// A failed restore is not a usage mistake; don't bury its output.
	SilenceUsage: true,
	RunE:         runRestore,
}

func init() {
	rootCmd.AddCommand(restoreCmd)
	restoreCmd.Flags().
		BoolVarP(&restoreYes, "yes", "y", false, "Skip the confirmation prompt")
}

func runRestore(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	service, file, err := restoreArgs(cfg, args)
	if err != nil {
		return err
	}

	dir := ""
	if file == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot find your home directory: %w. Pass the backup file", err)
		}
		dir = backupDir(home, cfg.Project, "")
	}

	return restore.Run(container.Local{}, restore.Options{
		Project:     cfg.Project,
		Service:     service,
		File:        file,
		Dir:         dir,
		ComposeFile: devComposeFile,
		Yes:         restoreYes,
		LogDir:      appLogDir,
		In:          os.Stdin,
		Out:         os.Stdout,
		ErrOut:      os.Stderr,
		Now:         time.Now,
	})
}

// restoreArgs splits [service] [file]. A single-service project may give just
// a file: one argument that is not the service's name is taken as the file.
func restoreArgs(cfg *config.Config, args []string) (string, string, error) {
	switch {
	case len(args) == 2:
		service, err := runningService(cfg, args[:1])
		return service, args[1], err
	case len(args) == 1 && !cfg.IsMonorepo() && args[0] != cfg.Service:
		service, err := runningService(cfg, nil)
		return service, args[0], err
	default:
		service, err := runningService(cfg, args)
		return service, "", err
	}
}
