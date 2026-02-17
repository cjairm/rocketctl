/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "rocketctl",
	Short: "Convention-based Docker orchestration CLI",
	Long: `RocketCTL is a convention-based CLI tool that orchestrates Docker image 
building, versioning, pushing, and deployment for any project.

It works by reading a minimal rocket.yaml config file and following folder 
structure conventions. Supports both monorepo and single-service repositories.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// No persistent flags needed for MVP
}
