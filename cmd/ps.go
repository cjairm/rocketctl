package cmd

import (
	"github.com/cjairm/rocketctl/internal/compose"
	"github.com/cjairm/rocketctl/internal/config"
	"github.com/spf13/cobra"
)

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List running containers",
	Long:  `Lists running containers for the current project.`,
	RunE:  runPS,
}

func init() {
	rootCmd.AddCommand(psCmd)
}

func runPS(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	return compose.PS(cfg.Project)
}
