package ssh

import (
	"os/exec"
	"strings"
	"testing"
)

func TestShellQuoteNeutralisesInjection(t *testing.T) {
	payloads := []string{
		"myapp",
		"my app",
		"a;touch /tmp/rocketctl-pwned",
		"$(id)",
		"a`id`",
		"it's",
	}
	for _, payload := range payloads {
		t.Run(payload, func(t *testing.T) {
			// Echo the quoted value through a real shell; it must come back
			// byte-for-byte with nothing executed or word-split.
			out, err := exec.Command("sh", "-c", "printf %s "+ShellQuote(payload)).Output()
			if err != nil {
				t.Fatalf("shell rejected the quoted value: %v", err)
			}
			if string(out) != payload {
				t.Errorf("round trip changed the value: got %q, want %q", string(out), payload)
			}
		})
	}
}

func TestShellQuoteKeepsTildeExpandable(t *testing.T) {
	// The '~' must stay OUTSIDE the quotes or the remote shell creates a
	// literal '~' directory instead of expanding to $HOME.
	remoteDir := "~/apps/" + ShellQuote("myapp")
	out, err := exec.Command("sh", "-c", "printf %s "+remoteDir).Output()
	if err != nil {
		t.Fatalf("shell rejected the path: %v", err)
	}
	if strings.HasPrefix(string(out), "~") {
		t.Errorf("tilde was not expanded: %q", string(out))
	}
	if !strings.HasSuffix(string(out), "/apps/myapp") {
		t.Errorf("unexpected expansion: %q", string(out))
	}
}
