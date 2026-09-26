package restore

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// fakeLocal stands in for this machine's docker. It answers the commands
// restore makes and records every one. It knows no ssh: restore only ever
// touches the local dev stack.
type fakeLocal struct {
	services   []string // services the dev compose file defines
	containers string   // `docker compose ps` output, one name per line
	labels     string   // `docker image inspect` labels JSON
	execOut    string
	execCode   int
	stdin      string // what the restore command received
	commands   []string
}

func (f *fakeLocal) ExecStream(command string, stdout, stderr io.Writer) (int, error) {
	return f.ExecStreamIn(command, nil, stdout, stderr)
}

func (f *fakeLocal) ExecStreamIn(
	command string,
	stdin io.Reader,
	stdout, stderr io.Writer,
) (int, error) {
	f.commands = append(f.commands, command)
	switch {
	case strings.HasPrefix(command, "docker compose "):
		// Like compose: a service the file does not define is an error, a
		// defined one that is stopped lists nothing.
		fields := strings.Fields(command)
		asked := strings.Trim(fields[len(fields)-1], "'")
		if !slices.Contains(f.services, asked) {
			_, _ = io.WriteString(stderr, "no such service: "+asked)
			return 1, nil
		}
		_, _ = io.WriteString(stdout, f.containers)
	case strings.HasPrefix(command, "docker inspect "):
		_, _ = io.WriteString(stdout, "sha256:dev\tmyapp-api-dev:latest\n")
	case strings.HasPrefix(command, "docker image inspect "):
		_, _ = io.WriteString(stdout, f.labels+"\n")
	case strings.HasPrefix(command, "docker exec "):
		if stdin != nil {
			data, _ := io.ReadAll(stdin)
			f.stdin = string(data)
		}
		_, _ = io.WriteString(stdout, f.execOut)
		return f.execCode, nil
	default:
		return 127, errors.New("fakeLocal: unexpected command " + command)
	}
	return 0, nil
}

func (f *fakeLocal) execCommands() []string {
	var out []string
	for _, c := range f.commands {
		if strings.HasPrefix(c, "docker exec ") {
			out = append(out, c)
		}
	}
	return out
}

const restoreLabels = `{"rocketctl.restore":"sh bin/restore.sh"}`

func newLocal() *fakeLocal {
	return &fakeLocal{
		services:   []string{"myapp-api", "myapp-web"},
		containers: "myapp-api-1\n",
		labels:     restoreLabels,
		execOut:    "restored\n",
	}
}

// newOptions returns options whose backup dir holds two backups of api; the
// newer one is "NEW-BACKUP".
func newOptions(t *testing.T) (Options, *bytes.Buffer) {
	t.Helper()
	var out bytes.Buffer
	root := t.TempDir()
	dir := filepath.Join(root, "backups")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"myapp-api-20260920-010101.tar.gz": "OLD-BACKUP",
		"myapp-api-20260924-010101.tar.gz": "NEW-BACKUP",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return Options{
		Project:     "myapp",
		Service:     "api",
		Dir:         dir,
		ComposeFile: "docker-compose.yml",
		Yes:         true,
		LogDir:      filepath.Join(root, "logs"),
		In:          strings.NewReader(""),
		Out:         &out,
		ErrOut:      &out,
		Now:         func() time.Time { return time.Date(2026, 9, 25, 14, 3, 7, 0, time.UTC) },
	}, &out
}

func TestRunFeedsTheNewestBackupToTheLocalContainer(t *testing.T) {
	local := newLocal()
	opts, _ := newOptions(t)

	if err := Run(local, opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := []string{"docker exec -i 'myapp-api-1' sh -c 'sh bin/restore.sh'"}
	if got := local.execCommands(); !slices.Equal(got, want) {
		t.Errorf("exec commands = %q, want %q", got, want)
	}
	if local.stdin != "NEW-BACKUP" {
		t.Errorf("restore command received %q, want the newest backup", local.stdin)
	}
}

func TestRunRestoresTheFileGiven(t *testing.T) {
	local := newLocal()
	opts, _ := newOptions(t)
	opts.File = filepath.Join(opts.Dir, "myapp-api-20260920-010101.tar.gz")

	if err := Run(local, opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if local.stdin != "OLD-BACKUP" {
		t.Errorf("restore command received %q, want the file given", local.stdin)
	}
}

func TestRunFindsTheContainerInTheLocalDevStack(t *testing.T) {
	local := newLocal()
	opts, _ := newOptions(t)

	if err := Run(local, opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	// The dev compose service is <project>-<service>, as for every command.
	want := "docker compose -f 'docker-compose.yml' ps --format '{{.Name}}' 'myapp-api'"
	if len(local.commands) == 0 || local.commands[0] != want {
		t.Errorf("commands = %q, want %q first", local.commands, want)
	}
	for _, c := range local.commands {
		if !strings.HasPrefix(c, "docker ") {
			t.Errorf("ran %q, want only local docker commands", c)
		}
	}
}

func TestRunNamesTheComposeServiceItLookedFor(t *testing.T) {
	local := newLocal()
	local.services = []string{"api"} // a dev file that does not follow the naming
	opts, _ := newOptions(t)

	err := Run(local, opts)
	if err == nil || !strings.Contains(err.Error(), "myapp-api") {
		t.Fatalf("Run() error = %v, want it to name the service myapp-api", err)
	}
	if got := local.execCommands(); len(got) != 0 {
		t.Errorf("ran %v, want nothing", got)
	}
}

func TestRunRefusesWhenTheDevStackIsNotRunning(t *testing.T) {
	local := newLocal()
	local.containers = ""
	opts, _ := newOptions(t)

	err := Run(local, opts)
	if err == nil || !strings.Contains(err.Error(), "rocketctl up") {
		t.Fatalf("Run() error = %v, want it to name 'rocketctl up'", err)
	}
	if got := local.execCommands(); len(got) != 0 {
		t.Errorf("ran %v, want nothing", got)
	}
}

func TestRunRefusesWhenSeveralContainersMatch(t *testing.T) {
	local := newLocal()
	local.containers = "myapp-api-1\nmyapp-api-2\n"
	opts, _ := newOptions(t)

	err := Run(local, opts)
	if err == nil || !strings.Contains(err.Error(), "myapp-api-2") {
		t.Fatalf("Run() error = %v, want a refusal naming the containers", err)
	}
	if got := local.execCommands(); len(got) != 0 {
		t.Errorf("ran %v, want nothing", got)
	}
}

func TestRunWithoutLabelHasNothingToRestore(t *testing.T) {
	for _, labels := range []string{"null", `{"rocketctl.restore":" "}`} {
		local := newLocal()
		local.labels = labels
		opts, out := newOptions(t)
		opts.Yes = false

		if err := Run(local, opts); err != nil {
			t.Fatalf("%s: Run() error = %v, want nil", labels, err)
		}
		if !strings.Contains(out.String(), "nothing to restore") {
			t.Errorf("%s: output = %q, want it to say nothing to restore", labels, out.String())
		}
		if strings.Contains(out.String(), "(y/n)") {
			t.Errorf("%s: asked for confirmation with nothing to restore", labels)
		}
		if got := local.execCommands(); len(got) != 0 {
			t.Errorf("%s: ran %v, want nothing", labels, got)
		}
	}
}

func TestRunRefusesWithNoBackupToRestore(t *testing.T) {
	local := newLocal()
	opts, _ := newOptions(t)
	opts.Dir = t.TempDir()

	err := Run(local, opts)
	if err == nil || !strings.Contains(err.Error(), "rocketctl backup") {
		t.Fatalf("Run() error = %v, want it to name 'rocketctl backup'", err)
	}
	if got := local.execCommands(); len(got) != 0 {
		t.Errorf("ran %v, want nothing", got)
	}
}

func TestRunRefusesAnEmptyOrMissingFile(t *testing.T) {
	empty := filepath.Join(t.TempDir(), "empty.tar.gz")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{empty, filepath.Join(t.TempDir(), "missing.tar.gz")} {
		local := newLocal()
		opts, _ := newOptions(t)
		opts.File = file

		if err := Run(local, opts); err == nil || !strings.Contains(err.Error(), file) {
			t.Errorf("Run(%s) error = %v, want a refusal naming the file", file, err)
		}
		if got := local.execCommands(); len(got) != 0 {
			t.Errorf("ran %v, want nothing", got)
		}
	}
}

func TestRunAsksBeforeRestoring(t *testing.T) {
	local := newLocal()
	opts, out := newOptions(t)
	opts.Yes = false
	opts.In = strings.NewReader("y\n")
	newest := filepath.Join(opts.Dir, "myapp-api-20260924-010101.tar.gz")
	date := time.Date(2026, 9, 24, 1, 1, 1, 0, time.Local)
	if err := os.Chtimes(newest, date, date); err != nil {
		t.Fatal(err)
	}

	if err := Run(local, opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, want := range []string{newest, "10 B", "2026-09-24 01:01", "myapp-api-1", "(y/n)"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("confirmation %q does not show %q", out.String(), want)
		}
	}
	if got := local.execCommands(); len(got) != 1 {
		t.Errorf("exec commands = %q, want the restore after a yes", got)
	}
}

func TestRunDeclinedRestoresNothing(t *testing.T) {
	for _, answer := range []string{"n\n", "\n", ""} {
		local := newLocal()
		opts, out := newOptions(t)
		opts.Yes = false
		opts.In = strings.NewReader(answer)

		if err := Run(local, opts); err != nil {
			t.Fatalf("answer %q: Run() error = %v", answer, err)
		}
		if got := local.execCommands(); len(got) != 0 {
			t.Errorf("answer %q: ran %v, want nothing", answer, got)
		}
		if !strings.Contains(out.String(), "Cancelled") {
			t.Errorf("answer %q: output = %q, want it to say Cancelled", answer, out.String())
		}
	}
}

func TestRunPassesTheAppExitCodeThrough(t *testing.T) {
	local := newLocal()
	local.execCode = 3
	opts, _ := newOptions(t)

	err := Run(local, opts)
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Run() error = %v, want an *ExitError", err)
	}
	if exitErr.ExitCode() != 3 {
		t.Errorf("exit code = %d, want 3", exitErr.ExitCode())
	}
	if !strings.Contains(err.Error(), "failed") {
		t.Errorf("error %q does not say the restore failed", err)
	}
}

func TestRunStreamsAndLogsTheOutput(t *testing.T) {
	local := newLocal()
	opts, out := newOptions(t)

	if err := Run(local, opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(out.String(), "restored") {
		t.Errorf("output %q was not streamed", out.String())
	}
	log, err := os.ReadFile(filepath.Join(opts.LogDir, "restore-api-20260925-140307.log"))
	if err != nil {
		t.Fatalf("log not written: %v", err)
	}
	for _, want := range []string{"restored", "myapp-api-20260924-010101.tar.gz", "exit status 0"} {
		if !strings.Contains(string(log), want) {
			t.Errorf("log %q does not contain %q", log, want)
		}
	}
	if strings.Contains(string(log), "NEW-BACKUP") {
		t.Errorf("log %q contains backup data", log)
	}
}
