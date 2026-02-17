package compose

import (
	"fmt"
	"os"
	"os/exec"
)

// Up starts services using docker compose
func Up(composeFile string, services []string, build, noCache, detached bool) error {
	args := []string{"compose"}

	if composeFile != "" {
		args = append(args, "-f", composeFile)
	}

	args = append(args, "up")

	if detached {
		args = append(args, "-d")
	}

	if build {
		args = append(args, "--build")
	}

	if noCache && build {
		args = append(args, "--no-cache")
	}

	args = append(args, services...)

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// Down stops services using docker compose
func Down(composeFile string) error {
	args := []string{"compose"}

	if composeFile != "" {
		args = append(args, "-f", composeFile)
	}

	args = append(args, "down")

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// Logs shows logs for a service
func Logs(composeFile string, service string, follow bool) error {
	args := []string{"compose"}

	if composeFile != "" {
		args = append(args, "-f", composeFile)
	}

	args = append(args, "logs")

	if follow {
		args = append(args, "-f")
	}

	if service != "" {
		args = append(args, service)
	}

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// Pull pulls images for services
func Pull(composeFile string) error {
	args := []string{"compose"}

	if composeFile != "" {
		args = append(args, "-f", composeFile)
	}

	args = append(args, "pull")

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// Exec executes a command in a running container
func Exec(containerName string, command []string) error {
	args := []string{"exec", "-it", containerName}
	args = append(args, command...)

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// PS lists containers
func PS(project string) error {
	args := []string{"ps"}

	if project != "" {
		args = append(args, "--filter", fmt.Sprintf("name=%s", project))
	}

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
