package registry

import (
	"bytes"
	"fmt"
	"os/exec"
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
