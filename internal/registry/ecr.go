package registry

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// LoginECR authenticates Docker with AWS ECR
func LoginECR(registry, region string) error {
	fmt.Println("Authenticating with AWS ECR...")

	// Get ECR login password
	getPasswordCmd := exec.Command("aws", "ecr", "get-login-password", "--region", region)
	passwordOutput, err := getPasswordCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get ECR login password: %w", err)
	}

	// Docker login using the password
	loginCmd := exec.Command("docker", "login", "--username", "AWS", "--password-stdin", registry)
	loginCmd.Stdin = bytes.NewReader(passwordOutput)

	output, err := loginCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to login to Docker registry: %w\n%s", err, string(output))
	}

	fmt.Println("✓ Successfully authenticated with ECR")
	return nil
}

// CreateRepository creates an ECR repository with AES-256 encryption and mutable tags.
// If the repository already exists, it is skipped (idempotent).
func CreateRepository(name, region string) error {
	cmd := exec.Command(
		"aws", "ecr", "create-repository",
		"--repository-name", name,
		"--region", region,
		"--image-tag-mutability", "MUTABLE",
		"--encryption-configuration", "encryptionType=AES256",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "RepositoryAlreadyExistsException") {
			fmt.Printf("  Repository %q already exists, skipping\n", name)
			return nil
		}
		return fmt.Errorf("failed to create ECR repository %q: %w\n%s", name, err, string(output))
	}
	return nil
}
