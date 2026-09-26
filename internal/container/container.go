// Package container finds a service's running container on the deploy host
// and runs an app-declared command in it. It is the shared ground of commands
// that follow "the app declares, rocketctl runs": the app names the command in
// an image label, and rocketctl knows nothing else about it.
package container

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/cjairm/rocketctl/internal/ssh"
)

// Runner runs a shell command on the deploy host. The returned int is the
// command's exit status; err is reserved for failing to run it at all.
type Runner interface {
	ExecStream(command string, stdout, stderr io.Writer) (int, error)
}

// Target is a service's one running container.
type Target struct {
	Name     string
	ImageID  string
	ImageRef string            // the tag the container was started from
	Labels   map[string]string // the image's labels, never nil
}

// Find returns the one running container of the project's service on host,
// with its image's labels. It refuses when none is running or several are.
func Find(r Runner, host, project, service string) (*Target, error) {
	// The compose service name the generated docker-compose.prod.yml gives
	// each service.
	svc := fmt.Sprintf("%s-%s", project, service)
	out, err := Query(r, fmt.Sprintf(
		"docker ps --filter %s --format %s",
		ssh.ShellQuote("label=com.docker.compose.service="+svc),
		ssh.ShellQuote("{{.Names}}"),
	))
	if err != nil {
		return nil, fmt.Errorf("failed to list containers on %s: %w", host, err)
	}
	names := strings.Fields(out)
	switch len(names) {
	case 0:
		return nil, fmt.Errorf(
			"service '%s' is not running on %s (no running container for compose service %s). Run 'rocketctl deploy' first",
			service,
			host,
			svc,
		)
	case 1:
	default:
		return nil, fmt.Errorf(
			"service '%s' has %d running containers on %s: %s. Scale it to one first",
			service, len(names), host, strings.Join(names, ", "),
		)
	}
	t := &Target{Name: names[0]}

	out, err = Query(r, fmt.Sprintf(
		"docker inspect --format %s %s",
		ssh.ShellQuote("{{.Image}}\t{{.Config.Image}}"),
		ssh.ShellQuote(t.Name),
	))
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container %s: %w", t.Name, err)
	}
	fields := strings.Split(strings.TrimSpace(out), "\t")
	if len(fields) != 2 {
		return nil, fmt.Errorf("unexpected docker inspect output for %s: %q", t.Name, out)
	}
	t.ImageID, t.ImageRef = fields[0], fields[1]

	// Read the labels from the image rather than the container, so they are
	// always what the app's Dockerfile declared.
	out, err = Query(r, fmt.Sprintf(
		"docker image inspect --format %s %s",
		ssh.ShellQuote("{{json .Config.Labels}}"),
		ssh.ShellQuote(t.ImageID),
	))
	if err != nil {
		return nil, fmt.Errorf("failed to inspect image %s: %w", t.ImageRef, err)
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &t.Labels); err != nil {
		return nil, fmt.Errorf("failed to parse labels of image %s: %w", t.ImageRef, err)
	}
	if t.Labels == nil {
		t.Labels = map[string]string{}
	}
	return t, nil
}

// Exec returns the host command that runs command in the named container. The
// command is the app's own shell text, so it is handed to the container's sh;
// quoted here so the host shell passes it through intact.
func Exec(name, command string) string {
	return fmt.Sprintf("docker exec %s sh -c %s", ssh.ShellQuote(name), ssh.ShellQuote(command))
}

// StartHint explains an exit status that usually means the command never
// started, or returns "" for any other status.
func StartHint(code int) string {
	// docker exec uses 125-127 for its own failures and sh uses 127 for a
	// missing program. The app may use them too, so this is a hint, not a
	// verdict.
	if code >= 125 && code <= 127 {
		return " (this status usually means the command could not start: container gone, no /bin/sh in the image, or the label's program not found)"
	}
	return ""
}

// Query runs a command whose stdout is data. Stderr is kept apart so a docker
// warning cannot corrupt what gets parsed.
func Query(r Runner, command string) (string, error) {
	var stdout, stderr bytes.Buffer
	code, err := r.ExecStream(command, &stdout, &stderr)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf(
			"%s exited with status %d: %s",
			command,
			code,
			strings.TrimSpace(stderr.String()),
		)
	}
	return stdout.String(), nil
}
