/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// binaryVersion is injected at build time with
// -ldflags "-X github.com/cjairm/rocketctl/cmd.binaryVersion=<version>".
// It stays "dev" for local `go build` and `go run`.
var binaryVersion = "dev"

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "rocketctl",
	Short: "Convention-based Docker orchestration CLI",
	Long: `RocketCTL is a convention-based CLI tool that orchestrates Docker image
building, versioning, pushing, and deployment for any project.

It works by reading a minimal rocket.yaml config file and following folder
structure conventions. Supports both monorepo and single-service repositories.`,
	// Version adds --version. It does not shadow the `version` subcommand,
	// which reports per-service versions from .rocket-version.
	Version: binaryVersion,
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
	rootCmd.SetVersionTemplate("rocketctl {{.Version}}\n")
}
