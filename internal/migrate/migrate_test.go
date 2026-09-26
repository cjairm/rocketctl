package migrate

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// fakeHost stands in for the remote server. It answers the docker queries
// migrate makes and records every command, so tests can check exactly what
// would have run over SSH.
type fakeHost struct {
	containers string // `docker ps` output, one name per line
	labels     string // `docker image inspect` labels JSON
	execOut    string // what the migration command prints
	execCode   int    // the migration command's exit code
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
		_, _ = io.WriteString(stdout, f.execOut)
		return f.execCode, nil
	default:
		return 127, errors.New("fakeHost: unexpected command " + command)
	}
	return 0, nil
}

// execCommands returns only the commands that ran something in a container.
func (f *fakeHost) execCommands() []string {
	var out []string
	for _, c := range f.commands {
		if strings.HasPrefix(c, "docker exec ") {
			out = append(out, c)
		}
	}
	return out
}

const migrateLabels = `{"rocketctl.migrate":"python bin/migrate.py","com.example":"x"}`

func newOptions(t *testing.T) (Options, *bytes.Buffer) {
	t.Helper()
	var out bytes.Buffer
	return Options{
		Host:    "deploy@10.0.0.5",
		Service: "api",
		Project: "myapp",
		LogDir:  t.TempDir(),
		In:      strings.NewReader(""),
		Out:     &out,
		ErrOut:  &out,
		Now:     func() time.Time { return time.Date(2026, 9, 25, 14, 3, 7, 0, time.UTC) },
	}, &out
}

func TestRunLooksUpTheComposeServiceContainer(t *testing.T) {
	host := &fakeHost{containers: "myapp-api\n", labels: migrateLabels}
	opts, _ := newOptions(t)

	if err := Run(host, opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := "docker ps --filter 'label=com.docker.compose.service=myapp-api' --format '{{.Names}}'"
	if len(host.commands) == 0 {
		t.Fatalf("ran no commands, want %q first", want)
	}
	if host.commands[0] != want {
		t.Errorf("first command = %q, want %q", host.commands[0], want)
	}
}

func TestRunWithoutLabelHasNothingToMigrate(t *testing.T) {
	for _, labels := range []string{"null", `{"com.example":"x"}`, `{"rocketctl.migrate":"  "}`} {
		t.Run(labels, func(t *testing.T) {
			host := &fakeHost{containers: "myapp-api\n", labels: labels}
			opts, out := newOptions(t)

			if err := Run(host, opts); err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}
			if !strings.Contains(out.String(), "nothing to migrate") {
				t.Errorf("output = %q, want it to say nothing to migrate", out.String())
			}
			if got := host.execCommands(); len(got) != 0 {
				t.Errorf("ran %v in the container, want nothing", got)
			}
			if entries, _ := os.ReadDir(opts.LogDir); len(entries) != 0 {
				t.Errorf("wrote %d log files, want none", len(entries))
			}
		})
	}
}

func TestRunRefusesWhenServiceIsNotRunning(t *testing.T) {
	host := &fakeHost{containers: "", labels: migrateLabels}
	opts, _ := newOptions(t)

	err := Run(host, opts)
	if err == nil || !strings.Contains(err.Error(), "not running") {
		t.Fatalf("Run() error = %v, want a 'not running' error", err)
	}
	if !strings.Contains(err.Error(), "api") {
		t.Errorf("error %q does not name the service", err)
	}
	if got := host.execCommands(); len(got) != 0 {
		t.Errorf("ran %v in a container, want nothing", got)
	}
}

func TestRunRefusesWhenSeveralContainersMatch(t *testing.T) {
	host := &fakeHost{containers: "myapp-api-1\nmyapp-api-2\n", labels: migrateLabels}
	opts, _ := newOptions(t)

	err := Run(host, opts)
	if err == nil {
		t.Fatal("Run() error = nil, want a refusal")
	}
	for _, name := range []string{"myapp-api-1", "myapp-api-2"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name container %s", err, name)
		}
	}
	if got := host.execCommands(); len(got) != 0 {
		t.Errorf("ran %v in a container, want nothing", got)
	}
}

func TestRunDefaultsToDryRun(t *testing.T) {
	host := &fakeHost{containers: "myapp-api\n", labels: migrateLabels}
	opts, out := newOptions(t)

	if err := Run(host, opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := []string{"docker exec 'myapp-api' sh -c 'python bin/migrate.py --dry-run'"}
	if got := host.execCommands(); !slices.Equal(got, want) {
		t.Errorf("exec commands = %q, want %q", got, want)
	}
	if strings.Contains(out.String(), "(y/n)") {
		t.Errorf("dry run asked for confirmation: %q", out.String())
	}
}

func TestRunApplyAsksBeforePassingApply(t *testing.T) {
	host := &fakeHost{containers: "myapp-api\n", labels: migrateLabels}
	opts, out := newOptions(t)
	opts.Apply = true
	opts.In = strings.NewReader("y\n")

	if err := Run(host, opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	prompt := out.String()
	for _, want := range []string{"deploy@10.0.0.5", "myapp-api", "reg.example.com/myapp_api:1.4.2", "(y/n)"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("confirmation %q does not show %q", prompt, want)
		}
	}
	want := []string{"docker exec 'myapp-api' sh -c 'python bin/migrate.py --apply'"}
	if got := host.execCommands(); !slices.Equal(got, want) {
		t.Errorf("exec commands = %q, want %q", got, want)
	}
}

func TestRunApplyDeclinedRunsNothing(t *testing.T) {
	for _, answer := range []string{"n\n", "\n", ""} {
		t.Run(strings.TrimSpace(answer), func(t *testing.T) {
			host := &fakeHost{containers: "myapp-api\n", labels: migrateLabels}
			opts, out := newOptions(t)
			opts.Apply = true
			opts.In = strings.NewReader(answer)

			if err := Run(host, opts); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := host.execCommands(); len(got) != 0 {
				t.Errorf("ran %v after answer %q, want nothing", got, answer)
			}
			if !strings.Contains(out.String(), "Cancelled") {
				t.Errorf("output = %q, want it to say Cancelled", out.String())
			}
		})
	}
}

func TestRunApplyWithYesSkipsThePrompt(t *testing.T) {
	host := &fakeHost{containers: "myapp-api\n", labels: migrateLabels}
	opts, out := newOptions(t)
	opts.Apply = true
	opts.Yes = true

	if err := Run(host, opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.Contains(out.String(), "(y/n)") {
		t.Errorf("--yes still asked: %q", out.String())
	}
	want := []string{"docker exec 'myapp-api' sh -c 'python bin/migrate.py --apply'"}
	if got := host.execCommands(); !slices.Equal(got, want) {
		t.Errorf("exec commands = %q, want %q", got, want)
	}
}

func TestRunPassesTheAppExitCodeThrough(t *testing.T) {
	host := &fakeHost{containers: "myapp-api\n", labels: migrateLabels, execCode: 3}
	opts, _ := newOptions(t)

	err := Run(host, opts)
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Run() error = %v, want an *ExitError", err)
	}
	if exitErr.Code != 3 {
		t.Errorf("exit code = %d, want 3", exitErr.Code)
	}
	for _, want := range []string{"failed", "3"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %q", err, want)
		}
	}
}

func TestRunStreamsAndLogsTheOutput(t *testing.T) {
	for _, code := range []int{0, 1} {
		t.Run(fmt.Sprintf("exit %d", code), func(t *testing.T) {
			host := &fakeHost{
				containers: "myapp-api\n",
				labels:     migrateLabels,
				execOut:    "step 1: would add column users.email\n",
				execCode:   code,
			}
			opts, out := newOptions(t)

			err := Run(host, opts)
			if !strings.Contains(out.String(), "step 1: would add column users.email") {
				t.Errorf("output %q was not streamed", out.String())
			}

			logPath := filepath.Join(opts.LogDir, "migrate-api-20260925-140307.log")
			data, readErr := os.ReadFile(logPath)
			if readErr != nil {
				t.Fatalf("log not written: %v", readErr)
			}
			for _, want := range []string{
				"step 1: would add column users.email",
				"python bin/migrate.py --dry-run",
				"reg.example.com/myapp_api:1.4.2",
				fmt.Sprintf("exit status %d", code),
			} {
				if !strings.Contains(string(data), want) {
					t.Errorf("log %q does not contain %q", data, want)
				}
			}
			if !strings.Contains(out.String(), logPath) {
				t.Errorf("output %q does not point to the log %s", out.String(), logPath)
			}
			if code != 0 && (err == nil || !strings.Contains(err.Error(), logPath)) {
				t.Errorf("failure error %v does not point to the log %s", err, logPath)
			}
		})
	}
}

func TestRunHandsTheLabelToTheContainerIntact(t *testing.T) {
	// App-controlled text must reach the container's sh as one argument and
	// never be interpreted by the host shell on the way.
	pwned := filepath.Join(t.TempDir(), "pwned")
	label := `echo "it's $(id)"; touch ` + pwned
	host := &fakeHost{
		containers: "myapp-api\n",
		labels:     fmt.Sprintf(`{"rocketctl.migrate":%q}`, label),
	}
	opts, _ := newOptions(t)
	if err := Run(host, opts); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	execs := host.execCommands()
	if len(execs) != 1 {
		t.Fatalf("exec commands = %q, want one", execs)
	}

	// A stand-in docker that prints each argument it receives on its own line.
	bin := t.TempDir()
	script := "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\"; done\n"
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", execs[0])
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("host shell rejected %q: %v", execs[0], err)
	}
	got := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	want := []string{"exec", "myapp-api", "sh", "-c", label + " --dry-run"}
	if !slices.Equal(got, want) {
		t.Errorf("docker received %q, want %q", got, want)
	}
	if _, err := os.Stat(pwned); err == nil {
		t.Errorf("the host shell executed part of the label: %s exists", pwned)
	}
}

func TestExitErrorHintsWhenTheCommandNeverStarted(t *testing.T) {
	// docker exec and sh use 125-127 when the command could not start at all,
	// which is not the same as a migration step failing.
	for _, code := range []int{125, 126, 127} {
		err := &ExitError{Code: code, Command: "python bin/migrate.py --dry-run", LogPath: "x.log"}
		if !strings.Contains(err.Error(), "could not start") {
			t.Errorf("status %d: error %q does not say the command could not start", code, err)
		}
	}
	for _, code := range []int{1, 2, 124, 128} {
		err := &ExitError{Code: code, Command: "python bin/migrate.py --dry-run", LogPath: "x.log"}
		if strings.Contains(err.Error(), "could not start") {
			t.Errorf("status %d: error %q blames startup for an app failure", code, err)
		}
	}
}
