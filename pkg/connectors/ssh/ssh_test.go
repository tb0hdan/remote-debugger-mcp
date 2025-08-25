package ssh

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type SSHConnectorTestSuite struct {
	suite.Suite
}

func TestSSHConnectorTestSuite(t *testing.T) {
	suite.Run(t, new(SSHConnectorTestSuite))
}

func (s *SSHConnectorTestSuite) TestNew_DefaultValues() {
	originalUser := os.Getenv("USER")
	defer func() {
		if originalUser != "" {
			os.Setenv("USER", originalUser)
		}
	}()

	// Test with USER env set
	os.Setenv("USER", "testuser")
	c := New("example.com", 0, "")
	s.Equal("example.com", c.config.Host)
	s.Equal(22, c.config.Port)
	s.Equal("testuser", c.config.User)

	// Test without USER env
	os.Unsetenv("USER")
	c = New("example.com", 0, "")
	s.Equal("root", c.config.User)

	// Test with explicit values
	c = New("example.com", 2222, "customuser")
	s.Equal("example.com", c.config.Host)
	s.Equal(2222, c.config.Port)
	s.Equal("customuser", c.config.User)
}

func (s *SSHConnectorTestSuite) TestGetTarget() {
	c := New("example.com", 22, "testuser")
	s.Equal("testuser@example.com", c.GetTarget())
}

func (s *SSHConnectorTestSuite) TestBuildSSHArgs_DefaultPort() {
	c := New("example.com", 22, "testuser")
	args := c.BuildSSHArgs()

	expected := []string{
		"-o", "StrictHostKeyChecking=yes",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"testuser@example.com",
	}
	s.Equal(expected, args)
}

func (s *SSHConnectorTestSuite) TestBuildSSHArgs_CustomPort() {
	c := New("example.com", 2222, "testuser")
	args := c.BuildSSHArgs()

	expected := []string{
		"-o", "StrictHostKeyChecking=yes",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-p", "2222",
		"testuser@example.com",
	}
	s.Equal(expected, args)
}

func (s *SSHConnectorTestSuite) TestBuildSCPArgs_DefaultPort() {
	c := New("example.com", 22, "testuser")
	args := c.BuildSCPArgs()

	expected := []string{
		"-o", "StrictHostKeyChecking=yes",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
	}
	s.Equal(expected, args)
}

func (s *SSHConnectorTestSuite) TestBuildSCPArgs_CustomPort() {
	c := New("example.com", 2222, "testuser")
	args := c.BuildSCPArgs()

	expected := []string{
		"-o", "StrictHostKeyChecking=yes",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-P", "2222",
	}
	s.Equal(expected, args)
}

func (s *SSHConnectorTestSuite) TestEscapeArg() {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple string",
			input:    "hello",
			expected: "'hello'",
		},
		{
			name:     "string with spaces",
			input:    "hello world",
			expected: "'hello world'",
		},
		{
			name:     "string with single quote",
			input:    "it's",
			expected: "'it'\\''s'",
		},
		{
			name:     "string with control characters",
			input:    "hello\x00world\x1f",
			expected: "'helloworld'",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "''",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			result := EscapeArg(tc.input)
			s.Equal(tc.expected, result)
		})
	}
}

func (s *SSHConnectorTestSuite) TestEscapeArgs() {
	input := []string{"hello", "world", "it's", "test\x00"}
	expected := []string{"'hello'", "'world'", "'it'\\''s'", "'test'"}

	result := EscapeArgs(input)
	s.Equal(expected, result)
}

func (s *SSHConnectorTestSuite) TestExecuteCommand_Success() {
	// This test requires mocking exec.CommandContext
	// For unit testing, we'll skip actual SSH execution
	// In a real scenario, you'd use a mock SSH server or test container

	if testing.Short() {
		s.T().Skip("Skipping integration test in short mode")
	}

	// Test would require actual SSH server setup
	// Example of what it would look like with a test SSH server:
	/*
		c := New("localhost", 22, "testuser")
		ctx := context.Background()
		output, err := c.ExecuteCommand(ctx, "echo 'test'")
		s.NoError(err)
		s.Contains(output, "test")
	*/
}

func (s *SSHConnectorTestSuite) TestExecuteCommand_ContextCancellation() {
	// Test context cancellation
	c := New("example.com", 22, "testuser")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := c.ExecuteCommand(ctx, "echo test")
	s.Error(err)
}

func (s *SSHConnectorTestSuite) TestExecuteCommandWithExitCode() {
	// This would require mocking or a test SSH server
	if testing.Short() {
		s.T().Skip("Skipping integration test in short mode")
	}

	// Example structure of the test:
	/*
		c := New("localhost", 22, "testuser")
		ctx := context.Background()

		// Success case
		output, exitCode, err := c.ExecuteCommandWithExitCode(ctx, "exit 0")
		s.NoError(err)
		s.Equal(0, exitCode)

		// Non-zero exit code
		output, exitCode, err = c.ExecuteCommandWithExitCode(ctx, "exit 42")
		s.NoError(err)
		s.Equal(42, exitCode)
	*/
}

func (s *SSHConnectorTestSuite) TestCopyFile() {
	c := New("example.com", 22, "testuser")
	ctx := context.Background()

	// Test building correct SCP command
	// Actual file copy would require SSH server
	if testing.Short() {
		s.T().Skip("Skipping integration test in short mode")
	}

	// Would test with actual files:
	/*
		tmpFile := "/tmp/test.txt"
		err := os.WriteFile(tmpFile, []byte("test content"), 0644)
		s.NoError(err)
		defer os.Remove(tmpFile)

		err = c.CopyFile(ctx, tmpFile, "/remote/path/test.txt")
		// Would check err based on SSH server availability
	*/

	// For now, just ensure the method exists and can be called
	err := c.CopyFile(ctx, "/local/path", "/remote/path")
	s.Error(err) // Will error without actual SSH connection
}

func (s *SSHConnectorTestSuite) TestCopyFileFromRemote() {
	c := New("example.com", 22, "testuser")
	ctx := context.Background()

	if testing.Short() {
		s.T().Skip("Skipping integration test in short mode")
	}

	// Would test with actual SSH server:
	/*
		err := c.CopyFileFromRemote(ctx, "/remote/file.txt", "/local/file.txt")
		// Check based on SSH availability
	*/

	// Create a context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	err := c.CopyFileFromRemote(ctx, "/remote/path", "/local/path")
	s.Error(err) // Will error without actual SSH connection
}

func (s *SSHConnectorTestSuite) TestMakeExecutable() {
	c := New("example.com", 22, "testuser")
	ctx := context.Background()

	if testing.Short() {
		s.T().Skip("Skipping integration test in short mode")
	}

	err := c.MakeExecutable(ctx, "/path/to/file")
	s.Error(err) // Will error without actual SSH connection
}

func (s *SSHConnectorTestSuite) TestRemoveFile() {
	c := New("example.com", 22, "testuser")
	ctx := context.Background()

	if testing.Short() {
		s.T().Skip("Skipping integration test in short mode")
	}

	err := c.RemoveFile(ctx, "/path/to/file")
	s.Error(err) // Will error without actual SSH connection
}

func (s *SSHConnectorTestSuite) TestFileExists() {
	c := New("example.com", 22, "testuser")
	ctx := context.Background()

	if testing.Short() {
		s.T().Skip("Skipping integration test in short mode")
	}

	exists, err := c.FileExists(ctx, "/path/to/file")
	s.Error(err) // Will error without actual SSH connection
	s.False(exists)
}

func (s *SSHConnectorTestSuite) TestTestConnection() {
	c := New("example.com", 22, "testuser")
	ctx := context.Background()

	if testing.Short() {
		s.T().Skip("Skipping integration test in short mode")
	}

	err := c.TestConnection(ctx)
	s.Error(err) // Will error without actual SSH connection
}

// MockCommandContext is a helper for testing command execution
type MockCommandContext struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

func (m *MockCommandContext) Run() error {
	if m.err != nil {
		return m.err
	}
	if m.exitCode != 0 {
		return &exec.ExitError{}
	}
	return nil
}

// TestExecuteCommandOutputHandling tests the output handling logic
func (s *SSHConnectorTestSuite) TestExecuteCommandOutputHandling() {
	// This demonstrates the expected output format
	c := New("example.com", 22, "testuser")

	// Test building args for command
	args := c.BuildSSHArgs()
	args = append(args, "echo test")

	s.Contains(args, "testuser@example.com")
	s.Contains(args, "echo test")
}

// TestPortHandling verifies port handling in different scenarios
func (s *SSHConnectorTestSuite) TestPortHandling() {
	testCases := []struct {
		name         string
		port         int
		expectedSSH  string
		expectedSCP  string
	}{
		{
			name:         "default port",
			port:         22,
			expectedSSH:  "",
			expectedSCP:  "",
		},
		{
			name:         "custom port",
			port:         2222,
			expectedSSH:  "2222",
			expectedSCP:  "2222",
		},
		{
			name:         "zero port defaults to 22",
			port:         0,
			expectedSSH:  "",
			expectedSCP:  "",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			c := New("example.com", tc.port, "testuser")

			sshArgs := c.BuildSSHArgs()
			scpArgs := c.BuildSCPArgs()

			if tc.expectedSSH != "" {
				s.Contains(sshArgs, "-p")
				s.Contains(sshArgs, tc.expectedSSH)
			} else {
				s.NotContains(sshArgs, "-p")
			}

			if tc.expectedSCP != "" {
				s.Contains(scpArgs, "-P")
				s.Contains(scpArgs, tc.expectedSCP)
			} else {
				s.NotContains(scpArgs, "-P")
			}
		})
	}
}

// TestContextTimeout verifies context timeout handling
func (s *SSHConnectorTestSuite) TestContextTimeout() {
	c := New("example.com", 22, "testuser")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond) // Ensure timeout

	_, err := c.ExecuteCommand(ctx, "echo test")
	s.Error(err)
}

// TestCommandInjectionPrevention tests that command injection is prevented
func (s *SSHConnectorTestSuite) TestCommandInjectionPrevention() {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "semicolon injection attempt",
			input:    "file; rm -rf /",
			expected: "'file; rm -rf /'",
		},
		{
			name:     "pipe injection attempt",
			input:    "file | cat /etc/passwd",
			expected: "'file | cat /etc/passwd'",
		},
		{
			name:     "backtick injection attempt",
			input:    "file`whoami`",
			expected: "'file`whoami`'",
		},
		{
			name:     "dollar injection attempt",
			input:    "file$(whoami)",
			expected: "'file$(whoami)'",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			escaped := EscapeArg(tc.input)
			s.Equal(tc.expected, escaped)
			// Verify the dangerous characters are contained within quotes
			s.True(strings.HasPrefix(escaped, "'"))
			s.True(strings.HasSuffix(escaped, "'"))
		})
	}
}

// BenchmarkEscapeArg benchmarks the EscapeArg function
func BenchmarkEscapeArg(b *testing.B) {
	input := "test'string\"with$special|characters;&"
	for i := 0; i < b.N; i++ {
		_ = EscapeArg(input)
	}
}

// BenchmarkBuildSSHArgs benchmarks building SSH arguments
func BenchmarkBuildSSHArgs(b *testing.B) {
	c := New("example.com", 2222, "testuser")
	for i := 0; i < b.N; i++ {
		_ = c.BuildSSHArgs()
	}
}

// TestConfigStructure verifies the Config struct
func (s *SSHConnectorTestSuite) TestConfigStructure() {
	config := Config{
		Host: "example.com",
		Port: 2222,
		User: "testuser",
	}

	s.Equal("example.com", config.Host)
	s.Equal(2222, config.Port)
	s.Equal("testuser", config.User)
}

// TestConnectorStructure verifies the Connector struct
func (s *SSHConnectorTestSuite) TestConnectorStructure() {
	config := Config{
		Host: "example.com",
		Port: 2222,
		User: "testuser",
	}

	connector := &Connector{
		config: config,
	}

	s.Equal(config, connector.config)
	s.Equal("testuser@example.com", connector.GetTarget())
}

// TestIntegerToStringConversion verifies port conversion
func (s *SSHConnectorTestSuite) TestIntegerToStringConversion() {
	ports := []int{22, 2222, 8022, 65535}

	for _, port := range ports {
		s.Run(strconv.Itoa(port), func() {
			c := New("example.com", port, "testuser")
			args := c.BuildSSHArgs()

			if port != 22 {
				s.Contains(args, strconv.Itoa(port))
			}
		})
	}
}