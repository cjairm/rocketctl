package docker

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// BuildOptions contains options for building a Docker image
type BuildOptions struct {
	ImageName  string
	Tag        string
	Dockerfile string
	Context    string
	BuildArgs  map[string]string
	NoCache    bool
}

// Build builds a Docker image
func Build(opts BuildOptions) error {
	args := []string{"build"}

	if opts.NoCache {
		args = append(args, "--no-cache")
	}

	// Add build args
	for key, value := range opts.BuildArgs {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", key, value))
	}

	// Add tags
	args = append(args, "-t", fmt.Sprintf("%s:%s", opts.ImageName, opts.Tag))

	// Add dockerfile
	if opts.Dockerfile != "" {
		args = append(args, "-f", opts.Dockerfile)
	}

	// Add context
	args = append(args, opts.Context)

	fmt.Printf("Building image: %s:%s\n", opts.ImageName, opts.Tag)
	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}

	fmt.Println("✓ Build successful")
	return nil
}

// Tag tags an existing image with a new tag
func Tag(sourceImage, sourceTag, targetImage, targetTag string) error {
	source := fmt.Sprintf("%s:%s", sourceImage, sourceTag)
	target := fmt.Sprintf("%s:%s", targetImage, targetTag)

	cmd := exec.Command("docker", "tag", source, target)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to tag image: %w", err)
	}

	fmt.Printf("✓ Tagged %s as %s\n", source, target)
	return nil
}

// Push pushes an image to a registry
func Push(image, tag string) error {
	fullImage := fmt.Sprintf("%s:%s", image, tag)
	fmt.Printf("Pushing image: %s\n", fullImage)

	cmd := exec.Command("docker", "push", fullImage)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker push failed: %w", err)
	}

	fmt.Println("✓ Push successful")
	return nil
}

// Run runs a container from an image
func Run(image, tag string, envFile string, detached bool, additionalArgs ...string) (string, error) {
	args := []string{"run"}

	if detached {
		args = append(args, "-d")
	}

	args = append(args, "--rm")

	if envFile != "" {
		args = append(args, "--env-file", envFile)
	}

	args = append(args, additionalArgs...)
	args = append(args, fmt.Sprintf("%s:%s", image, tag))

	cmd := exec.Command("docker", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("docker run failed: %w", err)
	}

	containerID := strings.TrimSpace(string(output))
	fmt.Printf("✓ Container started: %s\n", containerID)
	return containerID, nil
}

// ListImages lists Docker images matching a pattern
func ListImages(pattern string) ([]string, error) {
	cmd := exec.Command("docker", "images", "--format", "{{.Repository}}:{{.Tag}}", "--filter", fmt.Sprintf("reference=%s", pattern))
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	var images []string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			images = append(images, line)
		}
	}

	return images, nil
}

// RemoveImage removes a Docker image
func RemoveImage(image string) error {
	cmd := exec.Command("docker", "rmi", image)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove image %s: %w", image, err)
	}
	return nil
}

// LoadEnvFile loads a .env file and returns it as a map
func LoadEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open env file: %w", err)
	}
	defer file.Close()

	envVars := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			envVars[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading env file: %w", err)
	}

	return envVars, nil
}
