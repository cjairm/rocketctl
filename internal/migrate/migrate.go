// Package migrate runs an app's own release migrations inside its running
// container. The app declares the command with a Docker label; rocketctl knows
// nothing else about it.
package migrate

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cjairm/rocketctl/internal/ssh"
)

// Label is the image label an app sets to declare its migration command.
const Label = "rocketctl.migrate"

// Runner runs a shell command on the deploy host. The returned int is the
// command's exit status; err is reserved for failing to run it at all.
type Runner interface {
	ExecStream(command string, stdout, stderr io.Writer) (int, error)
}

// Options describes one migrate invocation.
type Options struct {
	Host    string // shown in the confirmation, e.g. "deploy@10.0.0.5"
	Project string
	Service string
	Apply   bool
	Yes     bool
	LogDir  string
	In      io.Reader
	Out     io.Writer
	ErrOut  io.Writer
	Now     func() time.Time
}

// ExitError reports that the app's migration command ran and failed. Code is
// the command's own exit status, for rocketctl to exit with.
type ExitError struct {
	Code    int
	Command string
	LogPath string
}

func (e *ExitError) Error() string {
	msg := fmt.Sprintf("migration failed: '%s' exited with status %d", e.Command, e.Code)
	// docker exec uses 125-127 for its own failures and sh uses 127 for a
	// missing program. The app may use them too, so this is a hint, not a
	// verdict, and the code is still passed through unchanged.
	if e.Code >= 125 && e.Code <= 127 {
		msg += " (this status usually means the command could not start: container gone, no /bin/sh in the image, or the label's program not found)"
	}
	return fmt.Sprintf("%s. Full output: %s", msg, e.LogPath)
}

// target is the running container migrate acts on.
type target struct {
	container string
	imageID   string
	imageRef  string // the tag the container was started from
	command   string // the label value, empty when the image declares none
}

// Run finds the service's running container, reads the migration command from
// its image and runs it in that container.
func Run(r Runner, opts Options) error {
	t, err := findTarget(r, opts)
	if err != nil {
		return err
	}
	if t.command == "" {
		_, _ = fmt.Fprintf(
			opts.Out,
			"✅ %s (%s) has no %s label: nothing to migrate\n",
			t.container, t.imageRef, Label,
		)
		return nil
	}

	// Exactly one mode flag, always. Anything short of an explicit --apply is
	// a dry run, so the app never sees a bare invocation from rocketctl.
	mode := "--dry-run"
	if opts.Apply {
		mode = "--apply"
	}
	command := t.command + " " + mode

	if opts.Apply && !opts.Yes {
		_, _ = fmt.Fprintln(opts.Out, "⚠️  About to APPLY migrations:")
		_, _ = fmt.Fprintf(opts.Out, "   Host:      %s\n", opts.Host)
		_, _ = fmt.Fprintf(opts.Out, "   Container: %s\n", t.container)
		_, _ = fmt.Fprintf(opts.Out, "   Image:     %s\n", t.imageRef)
		_, _ = fmt.Fprintf(opts.Out, "   Command:   %s\n", command)
		_, _ = fmt.Fprint(opts.Out, "\nProceed? (y/n): ")
		response, _ := bufio.NewReader(opts.In).ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			_, _ = fmt.Fprintln(opts.Out, "Cancelled")
			return nil
		}
	}

	logPath, logFile, err := createLog(opts)
	if err != nil {
		return err
	}
	defer func() { _ = logFile.Close() }()
	_, _ = fmt.Fprintf(
		logFile,
		"# %s\n# host:      %s\n# container: %s\n# image:     %s\n# command:   %s\n\n",
		opts.Now().Format(time.RFC3339),
		opts.Host,
		t.container,
		t.imageRef,
		command,
	)

	_, _ = fmt.Fprintf(opts.Out, "🔧 Running '%s' in %s (%s)...\n", command, t.container, t.imageRef)
	_, _ = fmt.Fprintf(opts.Out, "📝 Logging to %s\n", logPath)

	// stdout and stderr are copied on separate goroutines and both land in
	// the log, so the two tees share one lock.
	var mu sync.Mutex
	stdout := &lockedWriter{mu: &mu, w: io.MultiWriter(opts.Out, logFile)}
	stderr := &lockedWriter{mu: &mu, w: io.MultiWriter(opts.ErrOut, logFile)}

	// The label is the app's own shell command, so it is handed to the
	// container's sh; quoted here so the host shell passes it through intact.
	code, err := r.ExecStream(fmt.Sprintf(
		"docker exec %s sh -c %s",
		ssh.ShellQuote(t.container),
		ssh.ShellQuote(command),
	), stdout, stderr)
	if err != nil {
		_, _ = fmt.Fprintf(logFile, "\n# could not run the command: %v\n", err)
		return fmt.Errorf("failed to run migration in %s (log: %s): %w", t.container, logPath, err)
	}
	_, _ = fmt.Fprintf(logFile, "\n# exit status %d\n", code)
	if code != 0 {
		return &ExitError{Code: code, Command: command, LogPath: logPath}
	}

	if opts.Apply {
		_, _ = fmt.Fprintln(opts.Out, "✓ Migrations applied")
	} else {
		_, _ = fmt.Fprintln(
			opts.Out,
			"✓ Dry run complete. Re-run with --apply to make these changes",
		)
	}
	return nil
}

// createLog opens a fresh, timestamped log for this run. 0600 because
// migration output can carry production data.
func createLog(opts Options) (string, *os.File, error) {
	if err := os.MkdirAll(opts.LogDir, 0o755); err != nil {
		return "", nil, fmt.Errorf("failed to create log directory %s: %w", opts.LogDir, err)
	}
	name := fmt.Sprintf("migrate-%s-%s.log", opts.Service, opts.Now().Format("20060102-150405"))
	path := filepath.Join(opts.LogDir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create log %s: %w", path, err)
	}
	return path, f, nil
}

// lockedWriter serialises writes that share a destination.
type lockedWriter struct {
	mu *sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// composeService is the compose service name the generated
// docker-compose.prod.yml gives each service.
func composeService(opts Options) string {
	return fmt.Sprintf("%s-%s", opts.Project, opts.Service)
}

func findTarget(r Runner, opts Options) (*target, error) {
	svc := composeService(opts)
	out, err := query(r, fmt.Sprintf(
		"docker ps --filter %s --format %s",
		ssh.ShellQuote("label=com.docker.compose.service="+svc),
		ssh.ShellQuote("{{.Names}}"),
	))
	if err != nil {
		return nil, fmt.Errorf("failed to list containers on %s: %w", opts.Host, err)
	}
	names := strings.Fields(out)
	switch len(names) {
	case 0:
		return nil, fmt.Errorf(
			"service '%s' is not running on %s (no running container for compose service %s). Run 'rocketctl deploy' first",
			opts.Service,
			opts.Host,
			svc,
		)
	case 1:
	default:
		return nil, fmt.Errorf(
			"service '%s' has %d running containers on %s: %s. Scale it to one before migrating",
			opts.Service, len(names), opts.Host, strings.Join(names, ", "),
		)
	}
	t := &target{container: names[0]}

	out, err = query(r, fmt.Sprintf(
		"docker inspect --format %s %s",
		ssh.ShellQuote("{{.Image}}\t{{.Config.Image}}"),
		ssh.ShellQuote(t.container),
	))
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container %s: %w", t.container, err)
	}
	fields := strings.Split(strings.TrimSpace(out), "\t")
	if len(fields) != 2 {
		return nil, fmt.Errorf("unexpected docker inspect output for %s: %q", t.container, out)
	}
	t.imageID, t.imageRef = fields[0], fields[1]

	// Read the label from the image rather than the container, so it is
	// always what the app's Dockerfile declared.
	out, err = query(r, fmt.Sprintf(
		"docker image inspect --format %s %s",
		ssh.ShellQuote("{{json .Config.Labels}}"),
		ssh.ShellQuote(t.imageID),
	))
	if err != nil {
		return nil, fmt.Errorf("failed to inspect image %s: %w", t.imageRef, err)
	}
	var labels map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &labels); err != nil {
		return nil, fmt.Errorf("failed to parse labels of image %s: %w", t.imageRef, err)
	}
	t.command = strings.TrimSpace(labels[Label])
	return t, nil
}

// query runs a command whose stdout is data. Stderr is kept apart so a docker
// warning cannot corrupt what gets parsed.
func query(r Runner, command string) (string, error) {
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
