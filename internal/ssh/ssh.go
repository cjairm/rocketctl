package ssh

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

// Client represents an SSH connection
type Client struct {
	client *ssh.Client
}

// Connect establishes an SSH connection to the given host
// If keyPath is empty, it will look for default keys in ~/.ssh/
func Connect(host, user, keyPath string) (*Client, error) {
	authMethods, err := getAuthMethods(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get SSH auth methods: %w", err)
	}
	config := &ssh.ClientConfig{
		User: user,
		Auth: authMethods,
		// TODO: Consider using known_hosts
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
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
	defer session.Close()
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
	defer session.Close()
	// Set up output streams
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	if err := session.Run(command); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}
	return nil
}

// UploadFile uploads a local file to a remote path
func (c *Client) UploadFile(localPath, remotePath string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read local file %s: %w", localPath, err)
	}
	// Get file info for permissions
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("failed to stat local file %s: %w", localPath, err)
	}
	// Create remote directory if needed
	remoteDir := filepath.Dir(remotePath)
	if _, err := c.Exec(fmt.Sprintf("mkdir -p %s", remoteDir)); err != nil {
		return fmt.Errorf("failed to create remote directory %s: %w", remoteDir, err)
	}
	// Use SCP-like approach: write file content via cat
	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()
	// Set up stdin pipe
	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	// Start command to write file
	if err := session.Start(fmt.Sprintf("cat > %s && chmod %o %s", remotePath, info.Mode().Perm(), remotePath)); err != nil {
		return fmt.Errorf("failed to start upload command: %w", err)
	}
	// Write file content
	if _, err := io.Copy(stdin, strings.NewReader(string(data))); err != nil {
		return fmt.Errorf("failed to write file content: %w", err)
	}
	stdin.Close()
	// Wait for command to complete
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
	defer session.Close()

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
