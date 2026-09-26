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

// ListImageTags returns every tag in an ECR repository.
func ListImageTags(name, region string) ([]string, error) {
	cmd := exec.Command(
		"aws", "ecr", "list-images",
		"--repository-name", name,
		"--region", region,
		"--filter", "tagStatus=TAGGED",
		"--query", "imageIds[].imageTag",
		"--output", "text",
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list tags in ECR repository %q: %w", name, err)
	}
	return parseTagList(string(output)), nil
}

// DeleteImageTags removes tags from an ECR repository. An image whose last
// tag is removed is deleted. batch-delete-image exits 0 even when some tags
// fail, so its failures list is checked explicitly.
func DeleteImageTags(name, region string, tags []string) error {
	for _, batch := range imageIDBatches(tags) {
		args := []string{
			"ecr", "batch-delete-image",
			"--repository-name", name,
			"--region", region,
			"--query", "failures[].[imageId.imageTag,failureCode]",
			"--output", "text",
			"--image-ids",
		}
		cmd := exec.Command("aws", append(args, batch...)...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf(
				"failed to delete tags from ECR repository %q (needs ecr:BatchDeleteImage): %w\n%s",
				name, err, string(output),
			)
		}
		if err := deleteFailures(name, string(output)); err != nil {
			return err
		}
	}
	return nil
}

// parseTagList splits aws --output text, which separates values with tabs
// and newlines and prints "None" for an empty list.
func parseTagList(output string) []string {
	var tags []string
	for _, field := range strings.Fields(output) {
		if field != "None" {
			tags = append(tags, field)
		}
	}
	return tags
}

// imageIDBatches renders tags as --image-ids arguments, at most 100 per call
// (the batch-delete-image limit).
func imageIDBatches(tags []string) [][]string {
	const limit = 100
	var batches [][]string
	for start := 0; start < len(tags); start += limit {
		end := min(start+limit, len(tags))
		batch := make([]string, 0, end-start)
		for _, tag := range tags[start:end] {
			batch = append(batch, "imageTag="+tag)
		}
		batches = append(batches, batch)
	}
	return batches
}

// deleteFailures turns batch-delete-image's "tag<TAB>code" failure lines into
// an error, or nil when there are none.
func deleteFailures(name, output string) error {
	var failed []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] == "None" {
			continue
		}
		if len(fields) >= 2 {
			failed = append(failed, fmt.Sprintf("%s (%s)", fields[0], fields[1]))
		} else {
			failed = append(failed, fields[0])
		}
	}
	if len(failed) == 0 {
		return nil
	}
	return fmt.Errorf(
		"ECR repository %q could not delete: %s",
		name, strings.Join(failed, ", "),
	)
}
