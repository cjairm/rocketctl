package cmd

import (
	"fmt"

	"github.com/cjairm/rocketctl/internal/config"
	"github.com/cjairm/rocketctl/internal/registry"
	"github.com/spf13/cobra"
)

var ecrCmd = &cobra.Command{
	Use:   "ecr",
	Short: "Manage AWS ECR resources",
	Long: `Commands for managing AWS Elastic Container Registry resources, using your
local AWS CLI and the region in rocket.yaml.`,
	Example: `  rocketctl ecr create`,
}

var ecrCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create ECR repositories for all services",
	Long: `Creates an ECR repository for each service defined in rocket.yaml.

Repository names follow the convention <project>_<service>, matching the
image naming used by the build and push commands. Repositories that already
exist are skipped, so it is safe to run again after adding a service. New
repositories use AES-256 encryption and mutable tags.`,
	Example: `  rocketctl ecr create`,
	RunE:    runEcrCreate,
}

func init() {
	rootCmd.AddCommand(ecrCmd)
	ecrCmd.AddCommand(ecrCreateCmd)
}

func runEcrCreate(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fmt.Printf("Creating ECR repositories in region %s...\n", cfg.Region)

	for _, service := range cfg.GetServices() {
		repoName := cfg.GetImageName(service)
		fmt.Printf("Creating repository %q...\n", repoName)
		if err := registry.CreateRepository(repoName, cfg.Region); err != nil {
			return err
		}
		fmt.Printf("✓ Repository %q ready\n", repoName)
	}

	fmt.Println("\n✓ All ECR repositories created")
	return nil
}
