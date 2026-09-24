package ssh

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Client represents an SSH connection
type Client struct {
	client *ssh.Client
}

// hostKeyCallback verifies the server against ~/.ssh/known_hosts. An unknown
// host is recorded after the user confirms (trust on first use); a key that
// changed is always a hard failure, because that is what a MITM looks like.
func hostKeyCallback(insecure bool) (ssh.HostKeyCallback, error) {
	if insecure {
		fmt.Println("⚠️  Host key verification disabled (insecure_skip_host_key_check: true)")
		return ssh.InsecureIgnoreHostKey(), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to locate home directory for known_hosts: %w", err)
	}
	khPath := filepath.Join(home, ".ssh", "known_hosts")
	if err := os.MkdirAll(filepath.Dir(khPath), 0o700); err != nil {
		return nil, fmt.Errorf("failed to create %s: %w", filepath.Dir(khPath), err)
	}
	// knownhosts.New fails on a missing file, so make sure one exists.
	f, err := os.OpenFile(khPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", khPath, err)
	}
	_ = f.Close()

	verify, err := knownhosts.New(khPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", khPath, err)
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if err := verify(hostname, remote, key); err != nil {
			var keyErr *knownhosts.KeyError
			// Want is empty when the host is simply unknown; non-empty means
			// the recorded key did not match.
			if errors.As(err, &keyErr) && len(keyErr.Want) == 0 {
				return trustOnFirstUse(khPath, hostname, key)
			}
			return fmt.Errorf(
				"host key verification failed for %s: %w\n"+
					"If the server was rebuilt on purpose, remove its line from %s and deploy again",
				hostname, err, khPath,
			)
		}
		return nil
	}, nil
}

// trustOnFirstUse shows the fingerprint and records the key once the user
// types "yes", mirroring what OpenSSH does on a first connection.
func trustOnFirstUse(khPath, hostname string, key ssh.PublicKey) error {
	fmt.Printf("\nThe authenticity of host %s can't be established.\n", hostname)
	fmt.Printf("%s key fingerprint is %s\n", key.Type(), ssh.FingerprintSHA256(key))
	fmt.Print("Are you sure you want to continue connecting? (yes/no): ")

	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read confirmation: %w", err)
	}
	if strings.TrimSpace(strings.ToLower(answer)) != "yes" {
		return fmt.Errorf("host key not accepted, aborting deploy")
	}

	f, err := os.OpenFile(khPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open %s for append: %w", khPath, err)
	}
	defer func() { _ = f.Close() }()

	line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)
	if _, err := f.WriteString(line + "\n"); err != nil {
		return fmt.Errorf("failed to record host key in %s: %w", khPath, err)
	}
	fmt.Printf("✓ Added %s to %s\n", hostname, khPath)
	return nil
}

// Connect establishes an SSH connection to the given host
// If keyPath is empty, it will look for default keys in ~/.ssh/
func Connect(host, user, keyPath string, insecureHostKey bool) (*Client, error) {
	authMethods, err := getAuthMethods(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get SSH auth methods: %w", err)
	}
	hostKeys, err := hostKeyCallback(insecureHostKey)
	if err != nil {
		return nil, err
	}
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: hostKeys,
	}
	if !strings.Contains(host, ":") {
		host = host + ":22"
	}
	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", host, err)
	}
	return &Client{client: client}, nil
}

// Close closes the SSH connection
func (c *Client) Close() error {
	return c.client.Close()
}

// Exec executes a command on the remote server and returns the output
func (c *Client) Exec(command string) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer func() { _ = session.Close() }()
	output, err := session.CombinedOutput(command)
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}
	return string(output), nil
}

// ExecInteractive executes a command with stdout/stderr streaming
func (c *Client) ExecInteractive(command string) error {
	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer func() { _ = session.Close() }()
	// Set up output streams
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	if err := session.Run(command); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}
	return nil
}

// UploadFile uploads a local file to a remote path, preserving its mode.
func (c *Client) UploadFile(localPath, remotePath string) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("failed to stat local file %s: %w", localPath, err)
	}
	return c.UploadFileMode(localPath, remotePath, info.Mode().Perm())
}

// UploadFileMode uploads a local file and forces the remote permissions, which
// matters for secrets: a 0644 .env.example must not become a 0644 .env.
func (c *Client) UploadFileMode(localPath, remotePath string, mode os.FileMode) error {
	local, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to read local file %s: %w", localPath, err)
	}
	// Wrapped rather than a bare `defer local.Close()` so errcheck sees a
	// deliberate discard.
	defer func() { _ = local.Close() }()

	// Create remote directory if needed
	remoteDir := filepath.Dir(remotePath)
	if err := c.MkdirAll(remoteDir); err != nil {
		return err
	}

	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer func() { _ = session.Close() }()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Create the file with the right mode before any bytes land in it, so the
	// contents are never briefly readable by other users.
	cmd := fmt.Sprintf(
		"umask 077 && cat > %s && chmod %o %s",
		remotePath, mode, remotePath,
	)
	if err := session.Start(cmd); err != nil {
		return fmt.Errorf("failed to start upload command: %w", err)
	}
	if _, err := io.Copy(stdin, local); err != nil {
		return fmt.Errorf("failed to write file content: %w", err)
	}
	// Explicit discard, not a bare stdin.Close(): the remote command's real
	// outcome comes from session.Wait() below.
	_ = stdin.Close()

	if err := session.Wait(); err != nil {
		return fmt.Errorf("upload command failed: %w", err)
	}
	return nil
}

// MkdirAll creates a directory and all parent directories on the remote server
func (c *Client) MkdirAll(path string) error {
	if _, err := c.Exec(fmt.Sprintf("mkdir -p %s", path)); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}
	return nil
}

// FileExists checks if a file exists on the remote server
func (c *Client) FileExists(path string) (bool, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return false, fmt.Errorf("failed to create session: %w", err)
	}
	defer func() { _ = session.Close() }()

	// test -f returns exit code 0 if file exists, 1 if it doesn't
	err = session.Run(fmt.Sprintf("test -f %s", path))
	if err != nil {
		// Check if it's just a non-zero exit status (file doesn't exist)
		if _, ok := err.(*ssh.ExitError); ok {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}
	return true, nil
}

// getAuthMethods returns available SSH authentication methods
// If customKeyPath is provided, it will try that first
// Otherwise, it falls back to default SSH key locations
func getAuthMethods(customKeyPath string) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	// If custom key path is provided, try it first
	if customKeyPath != "" {
		// Expand ~ to home directory
		if strings.HasPrefix(customKeyPath, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to get home directory: %w", err)
			}
			customKeyPath = filepath.Join(home, customKeyPath[2:])
		}
		key, err := os.ReadFile(customKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read custom SSH key %s: %w", customKeyPath, err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("failed to parse custom SSH key %s: %w", customKeyPath, err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
		return methods, nil
	}
	// Try default SSH key locations
	home, err := os.UserHomeDir()
	if err == nil {
		// Try id_ed25519 (modern and recommended)
		keyPath := filepath.Join(home, ".ssh", "id_ed25519")
		if key, err := os.ReadFile(keyPath); err == nil {
			if signer, err := ssh.ParsePrivateKey(key); err == nil {
				methods = append(methods, ssh.PublicKeys(signer))
			}
		}
		// Try id_rsa (traditional)
		keyPath = filepath.Join(home, ".ssh", "id_rsa")
		if key, err := os.ReadFile(keyPath); err == nil {
			if signer, err := ssh.ParsePrivateKey(key); err == nil {
				methods = append(methods, ssh.PublicKeys(signer))
			}
		}
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf(
			"no SSH authentication methods available. Please ensure you have SSH keys set up (~/.ssh/id_rsa, ~/.ssh/id_ed25519) or specify a custom key path in rocket.yaml",
		)
	}
	return methods, nil
}

// ShellQuote wraps s in single quotes so a remote shell takes it as one
// literal argument. Embedded single quotes are closed, escaped and reopened.
//
// Callers must keep a leading "~" outside the quotes - a quoted tilde is not
// expanded, and the remote shell would create a literal "~" directory.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
