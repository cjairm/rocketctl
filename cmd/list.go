package cmd

import (
	"fmt"
	"strings"

	"github.com/cjairm/rocketctl/internal/config"
	"github.com/cjairm/rocketctl/internal/version"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all services",
	Long:  `Lists all services for the current project with their versions.`,
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fmt.Printf("Project: %s\n\n", cfg.Project)
	fmt.Printf("%-20s %-15s %s\n", "SERVICE", "VERSION", "DIRECTORY")
	fmt.Println(strings.Repeat("-", 60))

	for _, service := range cfg.GetServices() {
		versionPath, err := cfg.GetVersionFilePath(service)
		if err != nil {
			return err
		}

		ver, err := version.Get(versionPath)
		if err != nil {
			return err
		}

		serviceDir, err := cfg.GetServiceDirectory(service)
		if err != nil {
			return err
		}

		fmt.Printf("%-20s %-15s %s\n", service, ver, serviceDir)
	}

	return nil
}
