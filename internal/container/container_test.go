package container

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeHost stands in for the remote server and answers the docker queries
// Find makes, recording every command.
type fakeHost struct {
	containers string // `docker ps` output, one name per line
	labels     string // `docker image inspect` labels JSON
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
	default:
		return 127, errors.New("fakeHost: unexpected command " + command)
	}
	return 0, nil
}

func TestFindLooksUpTheComposeServiceContainer(t *testing.T) {
	host := &fakeHost{containers: "myapp-api\n", labels: `{"a":"b"}`}

	if _, err := Find(host, "deploy@10.0.0.5", "myapp", "api"); err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	want := "docker ps --filter 'label=com.docker.compose.service=myapp-api' --format '{{.Names}}'"
	if len(host.commands) == 0 || host.commands[0] != want {
		t.Errorf("commands = %q, want %q first", host.commands, want)
	}
}

func TestFindReturnsTheContainerImageAndLabels(t *testing.T) {
	host := &fakeHost{containers: "myapp-api\n", labels: `{"rocketctl.x":"run me","other":"y"}`}

	got, err := Find(host, "deploy@10.0.0.5", "myapp", "api")
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if got.Name != "myapp-api" {
		t.Errorf("Name = %q, want myapp-api", got.Name)
	}
	if got.ImageRef != "reg.example.com/myapp_api:1.4.2" {
		t.Errorf("ImageRef = %q, want the tag the container runs", got.ImageRef)
	}
	if got.Labels["rocketctl.x"] != "run me" {
		t.Errorf("Labels = %v, want rocketctl.x=run me", got.Labels)
	}
	// Labels are read from the image, not the container.
	if last := host.commands[len(host.commands)-1]; !strings.Contains(last, "'sha256:abc'") {
		t.Errorf("labels read with %q, want the image ID", last)
	}
}

func TestFindWithoutLabelsReturnsAnEmptySet(t *testing.T) {
	host := &fakeHost{containers: "myapp-api\n", labels: "null"}

	got, err := Find(host, "deploy@10.0.0.5", "myapp", "api")
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if got.Labels["anything"] != "" {
		t.Errorf("Labels = %v, want none", got.Labels)
	}
}

func TestFindRefusesWhenServiceIsNotRunning(t *testing.T) {
	host := &fakeHost{containers: ""}

	_, err := Find(host, "deploy@10.0.0.5", "myapp", "api")
	if err == nil || !strings.Contains(err.Error(), "not running") {
		t.Fatalf("Find() error = %v, want a 'not running' error", err)
	}
	if !strings.Contains(err.Error(), "rocketctl deploy") {
		t.Errorf("error %q does not name the fix", err)
	}
}

func TestFindRefusesWhenSeveralContainersMatch(t *testing.T) {
	host := &fakeHost{containers: "myapp-api-1\nmyapp-api-2\n"}

	_, err := Find(host, "deploy@10.0.0.5", "myapp", "api")
	if err == nil {
		t.Fatal("Find() error = nil, want a refusal")
	}
	for _, name := range []string{"myapp-api-1", "myapp-api-2"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name container %s", err, name)
		}
	}
}

func TestExecHandsTheCommandToTheContainerIntact(t *testing.T) {
	// App-controlled text must reach the container's sh as one argument and
	// never be interpreted by the host shell on the way.
	pwned := filepath.Join(t.TempDir(), "pwned")
	command := `echo "it's $(id)"; touch ` + pwned

	// A stand-in docker that prints each argument it receives on its own line.
	bin := t.TempDir()
	script := "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\"; done\n"
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", Exec("myapp-api", command))
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("host shell rejected %q: %v", Exec("myapp-api", command), err)
	}
	got := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	want := []string{"exec", "myapp-api", "sh", "-c", command}
	if !slices.Equal(got, want) {
		t.Errorf("docker received %q, want %q", got, want)
	}
	if _, err := os.Stat(pwned); err == nil {
		t.Errorf("the host shell executed part of the command: %s exists", pwned)
	}
}

func TestStartHint(t *testing.T) {
	// docker exec and sh use 125-127 when the command could not start at all,
	// which is not the same as the app's command failing.
	for _, code := range []int{125, 126, 127} {
		if !strings.Contains(StartHint(code), "could not start") {
			t.Errorf(
				"StartHint(%d) = %q, want it to say the command could not start",
				code,
				StartHint(code),
			)
		}
	}
	for _, code := range []int{0, 1, 2, 124, 128} {
		if got := StartHint(code); got != "" {
			t.Errorf("StartHint(%d) = %q, want no hint for an app failure", code, got)
		}
	}
}

func TestQueryKeepsStderrOutOfTheResult(t *testing.T) {
	r := runnerFunc(func(_ string, stdout, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stderr, "WARNING: noise\n")
		_, _ = io.WriteString(stdout, "data\n")
		return 0, nil
	})
	got, err := Query(r, "docker ps")
	if err != nil || got != "data\n" {
		t.Errorf("Query() = %q, %v; want %q, nil", got, err, "data\n")
	}

	failing := runnerFunc(func(_ string, _, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stderr, "permission denied")
		return 1, nil
	})
	_, err = Query(failing, "docker ps")
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("Query() error = %v, want the command's stderr", err)
	}
}

type runnerFunc func(string, io.Writer, io.Writer) (int, error)

func (f runnerFunc) ExecStream(c string, o, e io.Writer) (int, error) { return f(c, o, e) }

func TestInspectReadsTheImageAndItsLabels(t *testing.T) {
	host := &fakeHost{labels: `{"rocketctl.x":"run me"}`}

	got, err := Inspect(host, "myapp-api-1")
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if got.Name != "myapp-api-1" || got.ImageRef != "reg.example.com/myapp_api:1.4.2" {
		t.Errorf("Inspect() = %+v, want container myapp-api-1 on reg.example.com/myapp_api:1.4.2", got)
	}
	if got.Labels["rocketctl.x"] != "run me" {
		t.Errorf("Labels = %v, want rocketctl.x=run me", got.Labels)
	}
}

func TestImageLabels(t *testing.T) {
	host := &fakeHost{labels: "null"}

	got, err := ImageLabels(host, "reg.example.com/myapp_api:1.4.2")
	if err != nil {
		t.Fatalf("ImageLabels() error = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("ImageLabels() = %v, want an empty, non-nil set", got)
	}
	want := "docker image inspect --format '{{json .Config.Labels}}' 'reg.example.com/myapp_api:1.4.2'"
	if !slices.Equal(host.commands, []string{want}) {
		t.Errorf("commands = %q, want %q", host.commands, want)
	}
}

func TestLocalRunsOnThisMachine(t *testing.T) {
	var out, errOut strings.Builder
	code, err := Local{}.ExecStream("printf out; printf err >&2; exit 3", &out, &errOut)
	if err != nil {
		t.Fatalf("ExecStream() error = %v", err)
	}
	if code != 3 || out.String() != "out" || errOut.String() != "err" {
		t.Errorf("ExecStream() = %d, %q, %q; want 3, out, err", code, out.String(), errOut.String())
	}
}

func TestLocalFeedsStdin(t *testing.T) {
	var out strings.Builder
	code, err := Local{}.ExecStreamIn("cat", strings.NewReader("backup\x00bytes"), &out, io.Discard)
	if err != nil || code != 0 {
		t.Fatalf("ExecStreamIn() = %d, %v", code, err)
	}
	if out.String() != "backup\x00bytes" {
		t.Errorf("stdout = %q, want stdin passed through unchanged", out.String())
	}
}
