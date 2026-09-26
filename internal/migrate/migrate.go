// Package migrate runs an app's own release migrations inside its running
// container. The app declares the command with a Docker label; rocketctl knows
// nothing else about it.
package migrate

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cjairm/rocketctl/internal/container"
)

// Label is the image label an app sets to declare its migration command.
const Label = "rocketctl.migrate"

// Runner runs a shell command on the deploy host.
type Runner = container.Runner

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
	return fmt.Sprintf(
		"migration failed: '%s' exited with status %d%s. Full output: %s",
		e.Command, e.Code, container.StartHint(e.Code), e.LogPath,
	)
}

// ExitCode is the app's own exit status.
func (e *ExitError) ExitCode() int { return e.Code }

// Run finds the service's running container, reads the migration command from
// its image and runs it in that container.
func Run(r Runner, opts Options) error {
	t, err := container.Find(r, opts.Host, opts.Project, opts.Service)
	if err != nil {
		return err
	}
	label := strings.TrimSpace(t.Labels[Label])
	if label == "" {
		_, _ = fmt.Fprintf(
			opts.Out,
			"✅ %s (%s) has no %s label: nothing to migrate\n",
			t.Name, t.ImageRef, Label,
		)
		return nil
	}

	// Exactly one mode flag, always. Anything short of an explicit --apply is
	// a dry run, so the app never sees a bare invocation from rocketctl.
	mode := "--dry-run"
	if opts.Apply {
		mode = "--apply"
	}
	command := label + " " + mode

	if opts.Apply && !opts.Yes {
		_, _ = fmt.Fprintln(opts.Out, "⚠️  About to APPLY migrations:")
		_, _ = fmt.Fprintf(opts.Out, "   Host:      %s\n", opts.Host)
		_, _ = fmt.Fprintf(opts.Out, "   Container: %s\n", t.Name)
		_, _ = fmt.Fprintf(opts.Out, "   Image:     %s\n", t.ImageRef)
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
		t.Name,
		t.ImageRef,
		command,
	)

	_, _ = fmt.Fprintf(opts.Out, "🔧 Running '%s' in %s (%s)...\n", command, t.Name, t.ImageRef)
	_, _ = fmt.Fprintf(opts.Out, "📝 Logging to %s\n", logPath)

	// stdout and stderr are copied on separate goroutines and both land in
	// the log, so the two tees share one lock.
	var mu sync.Mutex
	stdout := &container.LockedWriter{Mu: &mu, W: io.MultiWriter(opts.Out, logFile)}
	stderr := &container.LockedWriter{Mu: &mu, W: io.MultiWriter(opts.ErrOut, logFile)}

	code, err := r.ExecStream(container.Exec(t.Name, command), stdout, stderr)
	if err != nil {
		_, _ = fmt.Fprintf(logFile, "\n# could not run the command: %v\n", err)
		return fmt.Errorf("failed to run migration in %s (log: %s): %w", t.Name, logPath, err)
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
