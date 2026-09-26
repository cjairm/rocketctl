// Package restore loads a backup into the app's local dev stack, never the
// server. The app declares the command with a Docker label on its dev image;
// rocketctl feeds it the backup on stdin and knows nothing else about it.
package restore

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cjairm/rocketctl/internal/backup"
	"github.com/cjairm/rocketctl/internal/config"
	"github.com/cjairm/rocketctl/internal/container"
	"github.com/cjairm/rocketctl/internal/ssh"
)

// Label is the image label an app sets to declare its restore command.
const Label = "rocketctl.restore"

// Runner runs commands on this machine, with stdin when a command needs it.
type Runner interface {
	container.Runner
	ExecStreamIn(command string, stdin io.Reader, stdout, stderr io.Writer) (int, error)
}

// Options describes one restore invocation.
type Options struct {
	Project     string
	Service     string // the rocket.yaml service; compose names it <project>-<service>
	File        string // the backup to load; "" means the newest in Dir
	Dir         string // where backups are saved
	ComposeFile string // the dev stack's compose file
	Yes         bool
	LogDir      string
	In          io.Reader
	Out         io.Writer
	ErrOut      io.Writer
	Now         func() time.Time
}

// ExitError reports that the app's restore command ran and failed. Code is
// the command's own exit status, for rocketctl to exit with.
type ExitError struct {
	Code    int
	Command string
	LogPath string
}

func (e *ExitError) Error() string {
	return fmt.Sprintf(
		"restore failed: '%s' exited with status %d%s. Full output: %s",
		e.Command, e.Code, container.StartHint(e.Code), e.LogPath,
	)
}

// ExitCode is the app's own exit status.
func (e *ExitError) ExitCode() int { return e.Code }

// Run finds the service's running dev container, reads the restore command
// from its image and runs it there with the backup on stdin.
func Run(r Runner, opts Options) error {
	t, err := findDevContainer(r, opts)
	if err != nil {
		return err
	}
	command := strings.TrimSpace(t.Labels[Label])
	if command == "" {
		_, _ = fmt.Fprintf(
			opts.Out,
			"✅ %s (%s) has no %s label: nothing to restore. Add it to the dev Dockerfile\n",
			t.Name, t.ImageRef, Label,
		)
		return nil
	}

	file := opts.File
	if file == "" {
		file, err = backup.Newest(opts.Dir, opts.Project, opts.Service)
		if err != nil {
			return err
		}
		if file == "" {
			return fmt.Errorf(
				"no backups of %s in %s. Run 'rocketctl backup %s' first, or pass a file",
				opts.Service, opts.Dir, opts.Service,
			)
		}
	}
	info, err := os.Stat(file)
	if err != nil {
		return fmt.Errorf("backup %s not found: %w", file, err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("backup %s is empty or not a file: nothing to restore from it", file)
	}

	if !opts.Yes {
		_, _ = fmt.Fprintln(opts.Out, "⚠️  About to REPLACE the local data of:")
		_, _ = fmt.Fprintf(opts.Out, "   Container: %s\n", t.Name)
		_, _ = fmt.Fprintf(opts.Out, "   Image:     %s\n", t.ImageRef)
		_, _ = fmt.Fprintf(opts.Out, "   Command:   %s\n", command)
		_, _ = fmt.Fprintln(opts.Out, "with the backup:")
		_, _ = fmt.Fprintf(opts.Out, "   File:      %s\n", file)
		_, _ = fmt.Fprintf(opts.Out, "   Size:      %s\n", backup.HumanSize(info.Size()))
		_, _ = fmt.Fprintf(
			opts.Out,
			"   Date:      %s\n",
			info.ModTime().Format("2006-01-02 15:04:05"),
		)
		_, _ = fmt.Fprint(opts.Out, "\nProceed? (y/n): ")
		response, _ := bufio.NewReader(opts.In).ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			_, _ = fmt.Fprintln(opts.Out, "Cancelled")
			return nil
		}
	}

	in, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("failed to open backup %s: %w", file, err)
	}
	defer func() { _ = in.Close() }()

	logPath, logFile, err := createLog(opts)
	if err != nil {
		return err
	}
	defer func() { _ = logFile.Close() }()
	_, _ = fmt.Fprintf(
		logFile,
		"# %s\n# container: %s\n# image:     %s\n# command:   %s\n# file:      %s\n\n",
		opts.Now().Format(time.RFC3339), t.Name, t.ImageRef, command, file,
	)

	_, _ = fmt.Fprintf(opts.Out, "♻️  Restoring %s into %s (%s)...\n", file, t.Name, t.ImageRef)
	_, _ = fmt.Fprintf(opts.Out, "📝 Logging to %s\n", logPath)

	// stdout and stderr are copied on separate goroutines and both land in
	// the log, so the two tees share one lock.
	var mu sync.Mutex
	stdout := &container.LockedWriter{Mu: &mu, W: io.MultiWriter(opts.Out, logFile)}
	stderr := &container.LockedWriter{Mu: &mu, W: io.MultiWriter(opts.ErrOut, logFile)}
	code, err := r.ExecStreamIn(container.ExecInput(t.Name, command), in, stdout, stderr)
	if err != nil {
		_, _ = fmt.Fprintf(logFile, "\n# could not run the command: %v\n", err)
		return fmt.Errorf("failed to run restore in %s (log: %s): %w", t.Name, logPath, err)
	}
	_, _ = fmt.Fprintf(logFile, "\n# exit status %d\n", code)
	if code != 0 {
		return &ExitError{Code: code, Command: command, LogPath: logPath}
	}
	_, _ = fmt.Fprintf(opts.Out, "✓ Restored %s into %s\n", filepath.Base(file), t.Name)
	return nil
}

// findDevContainer returns the service's one running container in the local
// dev stack. Compose scopes the lookup to this project's stack.
func findDevContainer(r Runner, opts Options) (*container.Target, error) {
	svc := config.ComposeServiceName(opts.Project, opts.Service)
	out, err := container.Query(r, fmt.Sprintf(
		"docker compose -f %s ps --format %s %s",
		ssh.ShellQuote(opts.ComposeFile),
		ssh.ShellQuote("{{.Name}}"),
		ssh.ShellQuote(svc),
	))
	if err != nil {
		return nil, fmt.Errorf(
			"failed to find compose service %s in %s (every service is named <project>-<service>): %w",
			svc, opts.ComposeFile, err,
		)
	}
	names := strings.Fields(out)
	switch len(names) {
	case 0:
		return nil, fmt.Errorf(
			"service '%s' is not running in the local dev stack (no running container for compose service %s in %s). Run 'rocketctl up' first",
			opts.Service, svc, opts.ComposeFile,
		)
	case 1:
	default:
		return nil, fmt.Errorf(
			"service '%s' has %d running containers locally: %s. Scale it to one first",
			opts.Service, len(names), strings.Join(names, ", "),
		)
	}
	return container.Inspect(r, names[0])
}

// createLog opens a fresh, timestamped log for this run. 0600 because the
// app's messages can carry data.
func createLog(opts Options) (string, *os.File, error) {
	if err := os.MkdirAll(opts.LogDir, 0o755); err != nil {
		return "", nil, fmt.Errorf("failed to create log directory %s: %w", opts.LogDir, err)
	}
	name := fmt.Sprintf("restore-%s-%s.log", opts.Service, opts.Now().Format("20060102-150405"))
	path := filepath.Join(opts.LogDir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create log %s: %w", path, err)
	}
	return path, f, nil
}
