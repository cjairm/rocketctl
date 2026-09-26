// Package backup saves an app's own backup, taken inside its running
// container, to a local file. The app declares the command with a Docker
// label; rocketctl knows nothing else about it.
package backup

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/cjairm/rocketctl/internal/container"
)

const (
	// Label is the image label an app sets to declare its backup command.
	Label = "rocketctl.backup"
	// SuffixLabel is the image label that sets the saved file's extension.
	SuffixLabel = "rocketctl.backup.suffix"
	// DefaultSuffix is used when the image sets no SuffixLabel.
	DefaultSuffix = ".backup"
)

// timestamp names each backup; it sorts in the order backups were taken.
const timestamp = "20060102-150405"

// partialExt marks a backup still being written. Only a complete, successful
// backup is renamed to its final name, so an interrupted run can never be
// mistaken for a backup or counted by --keep.
const partialExt = ".partial"

// validSuffix keeps an image-supplied suffix to a plain extension: it becomes
// part of a local file name, so it must not carry a path.
var validSuffix = regexp.MustCompile(`^(\.[A-Za-z0-9]+)+$`)

const maxSuffixLen = 16

// DefaultProgressEvery is how often progress is reported while a backup
// arrives, so a large one does not look like a hang.
const DefaultProgressEvery = 10 << 20

// Options describes one backup invocation.
type Options struct {
	Host    string // shown in the report, e.g. "deploy@10.0.0.5"
	Project string
	Service string
	Dir     string // where backups are saved
	Keep    int    // how many backups of this service to keep, at least 1
	// ProgressEvery reports progress each time this many more bytes have
	// arrived; 0 means DefaultProgressEvery.
	ProgressEvery int64
	LogDir        string
	Out           io.Writer
	ErrOut        io.Writer
	Now           func() time.Time
}

// ExitError reports that the app's backup command ran and failed. Code is
// the command's own exit status, for rocketctl to exit with.
type ExitError struct {
	Code    int
	Command string
	LogPath string
}

func (e *ExitError) Error() string {
	return fmt.Sprintf(
		"backup failed: '%s' exited with status %d%s. Nothing was saved. Full output: %s",
		e.Command, e.Code, container.StartHint(e.Code), e.LogPath,
	)
}

// ExitCode is the app's own exit status.
func (e *ExitError) ExitCode() int { return e.Code }

// Run finds the service's running container, reads the backup command from
// its image, runs it there and saves its stdout to a file in opts.Dir.
func Run(r container.Runner, opts Options) error {
	if opts.Keep < 1 {
		return fmt.Errorf(
			"--keep must be at least 1, got %d: it would delete the backup just taken",
			opts.Keep,
		)
	}

	t, err := container.Find(r, opts.Host, opts.Project, opts.Service)
	if err != nil {
		return err
	}
	command := strings.TrimSpace(t.Labels[Label])
	if command == "" {
		_, _ = fmt.Fprintf(
			opts.Out,
			"✅ %s (%s) has no %s label: nothing to back up\n",
			t.Name, t.ImageRef, Label,
		)
		return nil
	}
	suffix, err := suffixOf(t)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(opts.Dir, 0o700); err != nil {
		return fmt.Errorf("failed to create backup directory %s: %w", opts.Dir, err)
	}
	prefix := fmt.Sprintf("%s-%s-", opts.Project, opts.Service)
	stamp := opts.Now().Format(timestamp)
	path := filepath.Join(opts.Dir, prefix+stamp+suffix)
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("backup %s already exists. Wait a second and run it again", path)
	}
	partial := path + partialExt
	file, err := os.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("failed to create backup file %s: %w", partial, err)
	}
	// Until the rename below, every way out discards the partial file.
	saved := false
	defer func() {
		if !saved {
			_ = file.Close()
			_ = os.Remove(partial)
		}
	}()

	logPath, logFile, err := createLog(opts, stamp)
	if err != nil {
		return err
	}
	defer func() { _ = logFile.Close() }()
	_, _ = fmt.Fprintf(
		logFile,
		"# %s\n# host:      %s\n# container: %s\n# image:     %s\n# command:   %s\n# file:      %s\n\n",
		opts.Now().Format(time.RFC3339),
		opts.Host,
		t.Name,
		t.ImageRef,
		command,
		path,
	)

	_, _ = fmt.Fprintf(opts.Out, "💾 Backing up %s on %s\n", opts.Service, opts.Host)
	_, _ = fmt.Fprintf(opts.Out, "   Container: %s\n", t.Name)
	_, _ = fmt.Fprintf(opts.Out, "   Image:     %s\n", t.ImageRef)
	_, _ = fmt.Fprintf(opts.Out, "   Command:   %s\n", command)
	_, _ = fmt.Fprintf(opts.Out, "📝 Logging to %s\n", logPath)

	// stdout is the backup and goes to the file alone, hashed and counted on
	// the way; stderr is the app's messages, shown live and logged.
	hash := sha256.New()
	every := opts.ProgressEvery
	if every <= 0 {
		every = DefaultProgressEvery
	}
	code, err := r.ExecStream(
		container.Exec(t.Name, command),
		io.MultiWriter(file, hash, &progress{out: opts.Out, every: every}),
		io.MultiWriter(opts.ErrOut, logFile),
	)
	if err != nil {
		_, _ = fmt.Fprintf(logFile, "\n# could not run the command: %v\n", err)
		return fmt.Errorf(
			"backup failed: could not run it in %s (log: %s). Nothing was saved: %w",
			t.Name,
			logPath,
			err,
		)
	}
	_, _ = fmt.Fprintf(logFile, "\n# exit status %d\n", code)
	if code != 0 {
		return &ExitError{Code: code, Command: command, LogPath: logPath}
	}

	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to write backup %s: %w", partial, err)
	}
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to write backup %s: %w", partial, err)
	}
	if info.Size() == 0 {
		_, _ = fmt.Fprintln(logFile, "# empty backup discarded")
		return fmt.Errorf(
			"backup failed: '%s' exited 0 but the backup was empty (nothing on stdout). Nothing was saved. Full output: %s",
			command,
			logPath,
		)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to write backup %s: %w", partial, err)
	}
	if err := os.Rename(partial, path); err != nil {
		return fmt.Errorf("failed to save backup %s: %w", path, err)
	}
	saved = true
	sum := "sha256 " + hex.EncodeToString(hash.Sum(nil))
	_, _ = fmt.Fprintf(logFile, "# saved %d bytes, %s\n", info.Size(), sum)
	_, _ = fmt.Fprintf(opts.Out, "✓ Backup saved: %s (%s)\n", path, humanSize(info.Size()))
	_, _ = fmt.Fprintf(opts.Out, "   %s\n", sum)

	return prune(opts, prefix, suffix)
}

// suffixOf returns the image's backup suffix, or DefaultSuffix when it sets
// none. It refuses anything but a plain extension.
func suffixOf(t *container.Target) (string, error) {
	suffix := strings.TrimSpace(t.Labels[SuffixLabel])
	if suffix == "" {
		return DefaultSuffix, nil
	}
	if len(suffix) > maxSuffixLen || !validSuffix.MatchString(suffix) {
		return "", fmt.Errorf(
			"image %s sets %s=%q: it must be a plain file extension like .tar.gz (a dot, then letters and digits, at most %d characters). Fix the label in the app's Dockerfile",
			t.ImageRef,
			SuffixLabel,
			suffix,
			maxSuffixLen,
		)
	}
	return suffix, nil
}

// prune deletes this service's backups beyond the newest opts.Keep. It only
// touches names rocketctl itself gives backups with this suffix, so another
// service sharing a prefix (api vs api-v2), older suffixes and partial files
// are never matched.
func prune(opts Options, prefix, suffix string) error {
	pattern := regexp.MustCompile(
		"^" + regexp.QuoteMeta(prefix) + `\d{8}-\d{6}` + regexp.QuoteMeta(suffix) + "$",
	)
	entries, err := os.ReadDir(opts.Dir)
	if err != nil {
		return fmt.Errorf(
			"backup saved, but failed to list %s to prune old ones: %w",
			opts.Dir,
			err,
		)
	}
	var backups []string
	for _, e := range entries {
		if e.Type().IsRegular() && pattern.MatchString(e.Name()) {
			backups = append(backups, e.Name())
		}
	}
	if len(backups) <= opts.Keep {
		return nil
	}
	// The timestamp makes name order oldest first.
	slices.Sort(backups)
	var errs []error
	for _, name := range backups[:len(backups)-opts.Keep] {
		path := filepath.Join(opts.Dir, name)
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
			continue
		}
		_, _ = fmt.Fprintf(opts.Out, "🧹 Removed old backup %s\n", path)
	}
	if len(errs) > 0 {
		return fmt.Errorf("backup saved, but failed to prune old ones: %w", errors.Join(errs...))
	}
	return nil
}

// createLog opens a fresh, timestamped log for this run. 0600 because the
// app's messages can carry production details.
func createLog(opts Options, stamp string) (string, *os.File, error) {
	if err := os.MkdirAll(opts.LogDir, 0o755); err != nil {
		return "", nil, fmt.Errorf("failed to create log directory %s: %w", opts.LogDir, err)
	}
	path := filepath.Join(opts.LogDir, fmt.Sprintf("backup-%s-%s.log", opts.Service, stamp))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create log %s: %w", path, err)
	}
	return path, f, nil
}

// progress counts bytes as they arrive and prints one line whenever a write
// crosses the next multiple of every.
type progress struct {
	out   io.Writer
	every int64
	total int64
	next  int64
}

func (p *progress) Write(b []byte) (int, error) {
	if p.next == 0 {
		p.next = p.every
	}
	p.total += int64(len(b))
	if p.total >= p.next {
		_, _ = fmt.Fprintf(p.out, "   … %s received\n", humanSize(p.total))
		p.next = (p.total/p.every + 1) * p.every
	}
	return len(b), nil
}

// humanSize formats a byte count for people, e.g. "15 B" or "3.2 MB".
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
