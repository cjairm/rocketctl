package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/cjairm/rocketctl/internal/config"
	"github.com/cjairm/rocketctl/internal/docker"
	"github.com/cjairm/rocketctl/internal/version"
	"github.com/spf13/cobra"
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Clean up old Docker images",
	Long:  `Removes Docker images for the current project that are not the current version.`,
	RunE:  runPrune,
}

func init() {
	rootCmd.AddCommand(pruneCmd)
}

func runPrune(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Get current versions for all services
	currentVersions := make(map[string]string)
	for _, service := range cfg.GetServices() {
		versionPath, err := cfg.GetVersionFilePath(service)
		if err != nil {
			return err
		}

		ver, err := version.Get(versionPath)
		if err != nil {
			return err
		}

		currentVersions[service] = ver
	}

	// build tags every image twice, bare and registry-prefixed, so both
	// patterns have to be listed or half the images are never pruned.
	patterns := []string{
		fmt.Sprintf("%s_*", cfg.Project),
		fmt.Sprintf("%s/%s_*", cfg.Registry, cfg.Project),
	}

	var images []string
	for _, pattern := range patterns {
		found, err := docker.ListImages(pattern)
		if err != nil {
			return err
		}
		images = append(images, found...)
	}

	if len(images) == 0 {
		fmt.Println("No images found to prune")
		return nil
	}

	// Every tag that must survive: both forms, for every service.
	var keep []string
	for service, ver := range currentVersions {
		imageName := cfg.GetImageName(service)
		keep = append(keep,
			fmt.Sprintf("%s:%s", imageName, ver),
			cfg.GetFullImageName(service, ver),
		)
	}

	var imagesToRemove []string
	for _, image := range images {
		if !docker.MatchesAnyCurrent(image, keep) {
			imagesToRemove = append(imagesToRemove, image)
		}
	}

	if len(imagesToRemove) == 0 {
		fmt.Println("No old images to prune")
		return nil
	}

	// Show images to be removed
	fmt.Println("The following images will be removed:")
	for _, image := range imagesToRemove {
		fmt.Printf("  - %s\n", image)
	}

	// Confirm
	fmt.Print("\nProceed with removal? (y/n): ")
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if response != "y" && response != "yes" {
		fmt.Println("Cancelled")
		return nil
	}

	// Remove images
	for _, image := range imagesToRemove {
		fmt.Printf("Removing %s...\n", image)
		if err := docker.RemoveImage(image); err != nil {
			fmt.Printf("Warning: failed to remove %s: %v\n", image, err)
		} else {
			fmt.Printf("✓ Removed %s\n", image)
		}
	}

	fmt.Println("\n✓ Prune complete")
	return nil
}
