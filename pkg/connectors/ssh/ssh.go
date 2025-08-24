package ssh

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Config holds SSH connection configuration.
type Config struct {
	Host string
	Port int
	User string
}

// Connector provides SSH connectivity functionality.
type Connector struct {
	config Config
}

// New creates a new SSH connector.
func New(host string, port int, user string) *Connector {
	if port == 0 {
		port = 22
	}
	if user == "" {
		user = os.Getenv("USER")
		if user == "" {
			// Fallback to root. For the brave.
			user = "root"
		}
	}
	return &Connector{
		config: Config{
			Host: host,
			Port: port,
			User: user,
		},
	}
}

// GetTarget returns the SSH target string.
func (c *Connector) GetTarget() string {
	return fmt.Sprintf("%s@%s", c.config.User, c.config.Host)
}

// BuildSSHArgs builds common SSH arguments.
func (c *Connector) BuildSSHArgs() []string {
	args := []string{
		"-o", "StrictHostKeyChecking=yes",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
	}
	const defaultSSHPort = 22
	if c.config.Port != defaultSSHPort {
		args = append(args, "-p", strconv.Itoa(c.config.Port))
	}
	args = append(args, c.GetTarget())
	return args
}

// BuildSCPArgs builds SCP arguments for file transfer.
func (c *Connector) BuildSCPArgs() []string {
	args := []string{
		"-o", "StrictHostKeyChecking=yes",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
	}
	const defaultSSHPort = 22
	if c.config.Port != defaultSSHPort {
		args = append(args, "-P", strconv.Itoa(c.config.Port))
	}
	return args
}

// ExecuteCommand executes a command on the remote host.
func (c *Connector) ExecuteCommand(command string) (string, error) {
	args := c.BuildSSHArgs()
	args = append(args, command)

	cmd := exec.Command("ssh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += "STDERR:\n" + stderr.String()
	}

	return output, err
}

// ExecuteCommandWithExitCode executes a command and returns output with exit code.
func (c *Connector) ExecuteCommandWithExitCode(command string) (string, int, error) {
	args := c.BuildSSHArgs()
	args = append(args, command)

	cmd := exec.Command("ssh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			// Only clear error for non-zero exit codes, not for actual execution failures
			// This preserves the exit code but doesn't hide real errors
		} else {
			// Real execution error (e.g., SSH connection failed)
			return "", -1, err
		}
	}

	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += "STDERR:\n" + stderr.String()
	}

	return output, exitCode, nil
}

// CopyFile copies a local file to the remote host.
func (c *Connector) CopyFile(localPath, remotePath string) error {
	args := c.BuildSCPArgs()
	args = append(args, localPath, fmt.Sprintf("%s:%s", c.GetTarget(), remotePath))

	cmd := exec.Command("scp", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy file: %v - %s", err, stderr.String())
	}
	return nil
}

// CopyFileFromRemote copies a file from the remote host to local.
func (c *Connector) CopyFileFromRemote(remotePath, localPath string) error {
	args := c.BuildSCPArgs()
	args = append(args, fmt.Sprintf("%s:%s", c.GetTarget(), remotePath), localPath)

	cmd := exec.Command("scp", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy file from remote: %v - %s", err, stderr.String())
	}
	return nil
}

// MakeExecutable makes a file executable on the remote host.
func (c *Connector) MakeExecutable(remotePath string) error {
	_, err := c.ExecuteCommand("chmod +x " + EscapeArg(remotePath))
	return err
}

// RemoveFile removes a file on the remote host.
func (c *Connector) RemoveFile(remotePath string) error {
	_, err := c.ExecuteCommand("rm -f " + EscapeArg(remotePath))
	return err
}

// FileExists checks if a file exists on the remote host.
func (c *Connector) FileExists(remotePath string) (bool, error) {
	output, err := c.ExecuteCommand(fmt.Sprintf("test -f %s && echo 'exists' || echo 'not exists'", EscapeArg(remotePath)))
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) == "exists", nil
}

// TestConnection tests the SSH connection.
func (c *Connector) TestConnection() error {
	_, err := c.ExecuteCommand("echo 'connection test'")
	return err
}

// EscapeArg escapes a single argument for shell execution.
// This prevents shell injection by properly escaping special characters.
func EscapeArg(arg string) string {
	// Replace single quotes with escaped version
	escaped := strings.ReplaceAll(arg, "'", "'\\''")
	// Reject any control characters to prevent more sophisticated attacks
	for _, r := range escaped {
		if r < 32 || r == 127 {
			// Log warning about control characters and sanitize
			escaped = strings.ReplaceAll(escaped, string(r), "")
		}
	}
	return fmt.Sprintf("'%s'", escaped)
}

// EscapeArgs escapes multiple arguments for shell execution.
func EscapeArgs(args []string) []string {
	escaped := make([]string, len(args))
	for i, arg := range args {
		escaped[i] = EscapeArg(arg)
	}
	return escaped
}
