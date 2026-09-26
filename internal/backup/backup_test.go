package backup

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// fakeHost stands in for the remote server. It answers the docker queries
// backup makes and records every command, so tests can check exactly what
// would have run over SSH.
type fakeHost struct {
	containers string   // `docker ps` output, one name per line
	labels     string   // `docker image inspect` labels JSON
	execOut    string   // what the backup command writes to stdout
	execErr    string   // what it writes to stderr
	execChunks []string // when set, stdout arrives in these writes instead of execOut
	execCode   int      // its exit code
	execFail   error    // failing to run it at all, e.g. a dropped connection
	commands   []string
}

func (f *fakeHost) ExecStream(command string, stdout, stderr io.Writer) (int, error) {
	f.commands = append(f.commands, command)
	switch {
	case strings.HasPrefix(command, "docker ps "):
		_, _ = io.WriteString(stdout, f.containers)
	case strings.HasPrefix(command, "docker inspect "):
		_, _ = io.WriteString(stdout, "sha256:abc\treg.example.com/myapp_api:1.4.2\n")
	case strings.HasPrefix(command, "docker image inspect "):
		_, _ = io.WriteString(stdout, f.labels+"\n")
	case strings.HasPrefix(command, "docker exec "):
		_, _ = io.WriteString(stderr, f.execErr)
		if f.execChunks == nil {
			_, _ = io.WriteString(stdout, f.execOut)
		}
		for _, c := range f.execChunks {
			_, _ = io.WriteString(stdout, c)
		}
		return f.execCode, f.execFail
	default:
		return 127, errors.New("fakeHost: unexpected command " + command)
	}
	return 0, nil
}

func (f *fakeHost) execCommands() []string {
	var out []string
	for _, c := range f.commands {
		if strings.HasPrefix(c, "docker exec ") {
			out = append(out, c)
		}
	}
	return out
}

const backupLabels = `{"rocketctl.backup":"sh bin/backup.sh","rocketctl.backup.suffix":".tar.gz"}`

// wantName is the file a backup taken at newOptions' clock is saved as.
const wantName = "myapp-api-20260925-140307.tar.gz"

func newHost() *fakeHost {
	return &fakeHost{
		containers: "myapp-api\n",
		labels:     backupLabels,
		execOut:    "BACKUP-BYTES\x00\x01\x02",
		execErr:    "reading 3 records\n",
	}
}

func newOptions(t *testing.T) (Options, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errOut bytes.Buffer
	root := t.TempDir()
	return Options{
		Host:    "deploy@10.0.0.5",
		Project: "myapp",
		Service: "api",
		Dir:     filepath.Join(root, "backups"),
		Keep:    5,
		LogDir:  filepath.Join(root, "logs"),
		Out:     &out,
		ErrOut:  &errOut,
		Now:     func() time.Time { return time.Date(2026, 9, 25, 14, 3, 7, 0, time.UTC) },
	}, &out, &errOut
}

// files lists the names in dir, or none when it does not exist.
func files(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func touch(t *testing.T, dir string, names ...string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunWithoutLabelHasNothingToBackUp(t *testing.T) {
	for _, labels := range []string{"null", `{"com.example":"x"}`, `{"rocketctl.backup":"  "}`} {
		t.Run(labels, func(t *testing.T) {
			host := newHost()
			host.labels = labels
			opts, out, _ := newOptions(t)

			if err := Run(host, opts); err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}
			if !strings.Contains(out.String(), "nothing to back up") {
				t.Errorf("output = %q, want it to say nothing to back up", out.String())
			}
			if got := host.execCommands(); len(got) != 0 {
				t.Errorf("ran %v in the container, want nothing", got)
			}
			if got := files(t, opts.Dir); len(got) != 0 {
				t.Errorf("backup dir holds %v, want nothing", got)
			}
			if got := files(t, opts.LogDir); len(got) != 0 {
				t.Errorf("log dir holds %v, want nothing", got)
			}
		})
	}
}

func TestRunRunsTheLabelCommandAsIs(t *testing.T) {
	host := newHost()
	opts, _, _ := newOptions(t)

	if err := Run(host, opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := []string{"docker exec 'myapp-api' sh -c 'sh bin/backup.sh'"}
	if got := host.execCommands(); !slices.Equal(got, want) {
		t.Errorf("exec commands = %q, want %q", got, want)
	}
}

func TestRunSavesStdoutPrivatelyWithTheSuffix(t *testing.T) {
	host := newHost()
	opts, _, _ := newOptions(t)

	if err := Run(host, opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	path := filepath.Join(opts.Dir, wantName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("backup not saved: %v", err)
	}
	if string(data) != host.execOut {
		t.Errorf("backup = %q, want exactly the command's stdout %q", data, host.execOut)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("backup permissions = %o, want 600", perm)
	}
	if got := files(t, opts.Dir); !slices.Equal(got, []string{wantName}) {
		t.Errorf("backup dir holds %v, want only %s", got, wantName)
	}
}

func TestRunCreatesAPrivateBackupDir(t *testing.T) {
	host := newHost()
	opts, _, _ := newOptions(t)

	if err := Run(host, opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	info, err := os.Stat(opts.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("backup dir permissions = %o, want 700", perm)
	}
}

func TestRunKeepsStderrOutOfTheBackup(t *testing.T) {
	host := newHost()
	opts, _, errOut := newOptions(t)

	if err := Run(host, opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(opts.Dir, wantName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "reading") {
		t.Errorf("backup %q contains the command's stderr", data)
	}
	if !strings.Contains(errOut.String(), "reading 3 records") {
		t.Errorf("stderr %q was not shown", errOut.String())
	}
	log, err := os.ReadFile(filepath.Join(opts.LogDir, "backup-api-20260925-140307.log"))
	if err != nil {
		t.Fatalf("log not written: %v", err)
	}
	if !strings.Contains(string(log), "reading 3 records") {
		t.Errorf("log %q does not contain the command's stderr", log)
	}
	if strings.Contains(string(log), "BACKUP-BYTES") {
		t.Errorf("log %q contains backup data", log)
	}
}

func TestRunReportsWhereAndWhatItBackedUp(t *testing.T) {
	host := newHost()
	opts, out, _ := newOptions(t)

	if err := Run(host, opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, want := range []string{
		"deploy@10.0.0.5",
		"myapp-api",
		"reg.example.com/myapp_api:1.4.2",
		filepath.Join(opts.Dir, wantName),
		"15 B",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output %q does not show %q", out.String(), want)
		}
	}
}

func TestRunReportsTheChecksum(t *testing.T) {
	host := newHost()
	opts, out, _ := newOptions(t)

	if err := Run(host, opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	sum := sha256.Sum256([]byte(host.execOut))
	want := "sha256 " + hex.EncodeToString(sum[:])
	if !strings.Contains(out.String(), want) {
		t.Errorf("output %q does not show %q", out.String(), want)
	}
	log, err := os.ReadFile(filepath.Join(opts.LogDir, "backup-api-20260925-140307.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), want) {
		t.Errorf("log %q does not record %q", log, want)
	}
}

func TestRunReportsProgressAsBytesArrive(t *testing.T) {
	host := newHost()
	host.execChunks = []string{"abcd", "e", "fghijklmn", "o"} // 4, 5, 14, 15 bytes in
	opts, out, _ := newOptions(t)
	opts.ProgressEvery = 4

	if err := Run(host, opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	var got []string
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.Contains(line, "received") {
			got = append(got, strings.TrimSpace(line))
		}
	}
	// A line when a write reaches a step exactly, and one line for a write
	// that crosses several steps at once.
	want := []string{"… 4 B received", "… 14 B received"}
	if !slices.Equal(got, want) {
		t.Errorf("progress lines = %q, want %q", got, want)
	}
}

func TestRunDefaultsTheProgressStep(t *testing.T) {
	host := newHost()
	opts, out, _ := newOptions(t)

	if err := Run(host, opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.Contains(out.String(), "received") {
		t.Errorf("output %q reports progress for 15 bytes, want none below the default step", out.String())
	}
}

func TestRunDefaultsTheSuffix(t *testing.T) {
	for _, labels := range []string{
		`{"rocketctl.backup":"sh bin/backup.sh"}`,
		`{"rocketctl.backup":"sh bin/backup.sh","rocketctl.backup.suffix":" "}`,
	} {
		t.Run(labels, func(t *testing.T) {
			host := newHost()
			host.labels = labels
			opts, _, _ := newOptions(t)

			if err := Run(host, opts); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			want := []string{"myapp-api-20260925-140307.backup"}
			if got := files(t, opts.Dir); !slices.Equal(got, want) {
				t.Errorf("backup dir holds %v, want %v", got, want)
			}
		})
	}
}

func TestRunRefusesAnUnsafeSuffix(t *testing.T) {
	// The suffix comes from the image and becomes part of a local file name.
	for _, suffix := range []string{
		"/../../.ssh/authorized_keys",
		"..",
		".a/b",
		`.a\b`,
		"tar",
		".tar.",
		".tar gz",
		".abcdefghijklmnopq",
	} {
		t.Run(suffix, func(t *testing.T) {
			host := newHost()
			host.labels = `{"rocketctl.backup":"sh bin/backup.sh","rocketctl.backup.suffix":"` +
				strings.ReplaceAll(suffix, `\`, `\\`) + `"}`
			opts, _, _ := newOptions(t)

			err := Run(host, opts)
			if err == nil || !strings.Contains(err.Error(), SuffixLabel) {
				t.Fatalf("Run() error = %v, want a refusal naming %s", err, SuffixLabel)
			}
			if got := host.execCommands(); len(got) != 0 {
				t.Errorf("ran %v in the container, want nothing", got)
			}
			if got := files(t, opts.Dir); len(got) != 0 {
				t.Errorf("backup dir holds %v, want nothing", got)
			}
		})
	}
}

func TestRunDiscardsAFailedBackup(t *testing.T) {
	host := newHost()
	host.execOut = "half a backu"
	host.execCode = 3
	opts, _, _ := newOptions(t)

	err := Run(host, opts)
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Run() error = %v, want an *ExitError", err)
	}
	if exitErr.Code != 3 {
		t.Errorf("exit code = %d, want 3", exitErr.Code)
	}
	if !strings.Contains(err.Error(), "failed") {
		t.Errorf("error %q does not say the backup failed", err)
	}
	if got := files(t, opts.Dir); len(got) != 0 {
		t.Errorf("backup dir holds %v after a failure, want nothing", got)
	}
}

func TestRunDiscardsAnEmptyBackup(t *testing.T) {
	host := newHost()
	host.execOut = ""
	opts, _, _ := newOptions(t)

	err := Run(host, opts)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("Run() error = %v, want an empty-backup failure", err)
	}
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		t.Errorf(
			"Run() error = %v, an empty backup is rocketctl's verdict, not the app's exit code",
			err,
		)
	}
	if got := files(t, opts.Dir); len(got) != 0 {
		t.Errorf("backup dir holds %v after a failure, want nothing", got)
	}
}

func TestRunDiscardsABackupCutOffMidway(t *testing.T) {
	host := newHost()
	host.execOut = "half a backu"
	host.execFail = errors.New("connection lost")
	opts, _, _ := newOptions(t)

	if err := Run(host, opts); err == nil {
		t.Fatal("Run() error = nil, want a failure")
	}
	if got := files(t, opts.Dir); len(got) != 0 {
		t.Errorf("backup dir holds %v after a failure, want nothing", got)
	}
}

func TestRunPrunesOlderBackupsOfTheService(t *testing.T) {
	host := newHost()
	opts, _, _ := newOptions(t)
	opts.Keep = 3
	others := []string{
		"myapp-api-v2-20260101-000000.tar.gz",   // another service sharing a prefix
		"myapp-web-20260101-000000.tar.gz",      // another service
		"myapp-api-20260101-000000.dump",        // an older suffix
		"myapp-api-20260101-000000.tar.gz.part", // not a finished backup
		"myapp-api-notes.tar.gz",                // not a backup name at all
	}
	touch(t, opts.Dir, others...)
	touch(
		t, opts.Dir,
		"myapp-api-20260920-010101.tar.gz",
		"myapp-api-20260921-010101.tar.gz",
		"myapp-api-20260922-010101.tar.gz",
		"myapp-api-20260923-010101.tar.gz",
	)

	if err := Run(host, opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := append([]string{
		"myapp-api-20260922-010101.tar.gz",
		"myapp-api-20260923-010101.tar.gz",
		wantName,
	}, others...)
	slices.Sort(want)
	if got := files(t, opts.Dir); !slices.Equal(got, want) {
		t.Errorf("backup dir holds\n %v\nwant\n %v", got, want)
	}
}

func TestRunKeepOneLeavesOnlyTheNewBackup(t *testing.T) {
	host := newHost()
	opts, _, _ := newOptions(t)
	opts.Keep = 1
	touch(t, opts.Dir, "myapp-api-20260920-010101.tar.gz")

	if err := Run(host, opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := files(t, opts.Dir); !slices.Equal(got, []string{wantName}) {
		t.Errorf("backup dir holds %v, want only %s", got, wantName)
	}
}

func TestRunAcceptsASuffixAtTheLengthLimit(t *testing.T) {
	suffix := ".abcdefghijklmno" // 16 characters
	host := newHost()
	host.labels = `{"rocketctl.backup":"sh bin/backup.sh","rocketctl.backup.suffix":"` + suffix + `"}`
	opts, _, _ := newOptions(t)

	if err := Run(host, opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := []string{"myapp-api-20260925-140307" + suffix}
	if got := files(t, opts.Dir); !slices.Equal(got, want) {
		t.Errorf("backup dir holds %v, want %v", got, want)
	}
}

func TestRunFailureNeverPrunes(t *testing.T) {
	host := newHost()
	host.execCode = 1
	opts, _, _ := newOptions(t)
	opts.Keep = 1
	old := []string{"myapp-api-20260920-010101.tar.gz", "myapp-api-20260921-010101.tar.gz"}
	touch(t, opts.Dir, old...)

	if err := Run(host, opts); err == nil {
		t.Fatal("Run() error = nil, want a failure")
	}
	if got := files(t, opts.Dir); !slices.Equal(got, old) {
		t.Errorf("backup dir holds %v after a failure, want %v untouched", got, old)
	}
}

func TestRunRefusesToKeepFewerThanOne(t *testing.T) {
	for _, keep := range []int{0, -1} {
		host := newHost()
		opts, _, _ := newOptions(t)
		opts.Keep = keep

		err := Run(host, opts)
		if err == nil || !strings.Contains(err.Error(), "--keep") {
			t.Errorf("keep %d: Run() error = %v, want a refusal naming --keep", keep, err)
		}
		if len(host.commands) != 0 {
			t.Errorf("keep %d: ran %v, want nothing", keep, host.commands)
		}
	}
}

func TestRunNeverOverwritesABackup(t *testing.T) {
	host := newHost()
	opts, _, _ := newOptions(t)
	touch(t, opts.Dir, wantName)

	if err := Run(host, opts); err == nil {
		t.Fatal("Run() error = nil, want a refusal")
	}
	data, err := os.ReadFile(filepath.Join(opts.Dir, wantName))
	if err != nil || string(data) != "old" {
		t.Errorf("existing backup = %q, %v; want it untouched", data, err)
	}
}

func TestExitErrorExitCode(t *testing.T) {
	err := &ExitError{Code: 4, Command: "sh bin/backup.sh", LogPath: "x.log"}
	if err.ExitCode() != 4 {
		t.Errorf("ExitCode() = %d, want 4", err.ExitCode())
	}
	if !strings.Contains(err.Error(), "x.log") {
		t.Errorf("error %q does not point to the log", err)
	}
}

func TestHumanSize(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{5 * 1024 * 1024, "5.0 MB"},
		{3 * 1024 * 1024 * 1024, "3.0 GB"},
	}
	for _, tt := range tests {
		if got := humanSize(tt.n); got != tt.want {
			t.Errorf("humanSize(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}
