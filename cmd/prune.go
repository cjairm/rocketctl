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

	// List all images for this project
	pattern := fmt.Sprintf("%s_*", cfg.Project)
	images, err := docker.ListImages(pattern)
	if err != nil {
		return err
	}

	if len(images) == 0 {
		fmt.Println("No images found to prune")
		return nil
	}

	// Filter out images that match current versions
	var imagesToRemove []string
	for _, image := range images {
		shouldRemove := true
		for service, ver := range currentVersions {
			imageName := cfg.GetImageName(service)
			currentImage := fmt.Sprintf("%s:%s", imageName, ver)
			if image == currentImage {
				shouldRemove = false
				break
			}
		}
		if shouldRemove {
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
