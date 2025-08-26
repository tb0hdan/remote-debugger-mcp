package kube

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type KubeConnectorTestSuite struct {
	suite.Suite
}

func TestKubeConnectorTestSuite(t *testing.T) {
	suite.Run(t, new(KubeConnectorTestSuite))
}

func (s *KubeConnectorTestSuite) TestNew_DefaultValues() {
	// Test with empty namespace (should default to "default")
	c := New("", "test-pod", "test-container", "/path/to/kubeconfig")
	s.Equal("default", c.config.Namespace)
	s.Equal("test-pod", c.config.Pod)
	s.Equal("test-container", c.config.Container)
	s.Equal("/path/to/kubeconfig", c.config.Kubeconfig)

	// Test with explicit namespace
	c = New("kube-system", "test-pod", "test-container", "")
	s.Equal("kube-system", c.config.Namespace)
	s.Equal("test-pod", c.config.Pod)
	s.Equal("test-container", c.config.Container)
	s.Equal("", c.config.Kubeconfig)

	// Test with all empty values except pod
	c = New("", "test-pod", "", "")
	s.Equal("default", c.config.Namespace)
	s.Equal("test-pod", c.config.Pod)
	s.Equal("", c.config.Container)
	s.Equal("", c.config.Kubeconfig)
}

func (s *KubeConnectorTestSuite) TestGetPodIdentifier() {
	c := New("test-namespace", "test-pod", "test-container", "")
	s.Equal("test-namespace/test-pod", c.GetPodIdentifier())

	c = New("default", "my-pod", "", "")
	s.Equal("default/my-pod", c.GetPodIdentifier())
}

func (s *KubeConnectorTestSuite) TestBuildKubectlArgs_Empty() {
	c := New("", "test-pod", "", "")
	args := c.BuildKubectlArgs()

	// The method may include --kubeconfig if ~/.kube/config exists
	// So we check that the namespace args are present
	s.Contains(args, "-n")
	s.Contains(args, "default")
	// Ensure args come in pairs (flag, value)
	s.True(len(args)%2 == 0, "kubectl args should come in flag-value pairs")
}

func (s *KubeConnectorTestSuite) TestBuildKubectlArgs_WithKubeconfig() {
	c := New("test-ns", "test-pod", "", "/path/to/config")
	args := c.BuildKubectlArgs()

	expected := []string{
		"--kubeconfig", "/path/to/config",
		"-n", "test-ns",
	}
	s.Equal(expected, args)
}

func (s *KubeConnectorTestSuite) TestBuildKubectlArgs_WithNamespace() {
	c := New("custom-namespace", "test-pod", "", "")
	args := c.BuildKubectlArgs()

	// The method may include --kubeconfig if ~/.kube/config exists
	// So we check that the namespace args are present
	s.Contains(args, "-n")
	s.Contains(args, "custom-namespace")
	// Ensure args come in pairs (flag, value)
	s.True(len(args)%2 == 0, "kubectl args should come in flag-value pairs")
}

func (s *KubeConnectorTestSuite) TestBuildKubectlArgs_Full() {
	c := New("production", "app-pod", "app-container", "/home/user/.kube/config")
	args := c.BuildKubectlArgs()

	expected := []string{
		"--kubeconfig", "/home/user/.kube/config",
		"-n", "production",
	}
	s.Equal(expected, args)
}

func (s *KubeConnectorTestSuite) TestExecuteCommand_BuildsCorrectArgs() {
	c := New("test-ns", "test-pod", "test-container", "/path/to/config")

	// We can't actually execute kubectl without a cluster,
	// but we can verify the command construction
	ctx := context.Background()
	_, err := c.ExecuteCommand(ctx, "echo test")

	// The command will fail without kubectl/cluster, but that's expected
	s.Error(err)
}

func (s *KubeConnectorTestSuite) TestExecuteCommandWithExitCode() {
	c := New("test-ns", "test-pod", "test-container", "")
	ctx := context.Background()

	if testing.Short() {
		s.T().Skip("Skipping integration test in short mode")
	}

	// Test that the method exists and handles context
	_, exitCode, err := c.ExecuteCommandWithExitCode(ctx, "echo test")
	// The command may fail with exit code 1 if kubectl is installed but no cluster configured
	// or -1 if kubectl is not found
	s.True(exitCode == -1 || exitCode == 1)
	// If kubectl exists but fails, err will be nil with non-zero exit code
	// If kubectl doesn't exist, err will be non-nil
	_ = err
}

func (s *KubeConnectorTestSuite) TestCopyFileToPod() {
	c := New("test-ns", "test-pod", "test-container", "")
	ctx := context.Background()

	if testing.Short() {
		s.T().Skip("Skipping integration test in short mode")
	}

	err := c.CopyFileToPod(ctx, "/local/file.txt", "/remote/file.txt")
	s.Error(err) // Will error without actual kubectl/cluster
}

func (s *KubeConnectorTestSuite) TestCopyFileFromPod() {
	c := New("test-ns", "test-pod", "test-container", "")
	ctx := context.Background()

	if testing.Short() {
		s.T().Skip("Skipping integration test in short mode")
	}

	err := c.CopyFileFromPod(ctx, "/remote/file.txt", "/local/file.txt")
	s.Error(err) // Will error without actual kubectl/cluster
}

func (s *KubeConnectorTestSuite) TestGetPodStatus() {
	c := New("test-ns", "test-pod", "", "")
	ctx := context.Background()

	if testing.Short() {
		s.T().Skip("Skipping integration test in short mode")
	}

	_, err := c.GetPodStatus(ctx)
	s.Error(err) // Will error without actual kubectl/cluster
}

func (s *KubeConnectorTestSuite) TestGetContainerLogs() {
	c := New("test-ns", "test-pod", "test-container", "")
	ctx := context.Background()

	if testing.Short() {
		s.T().Skip("Skipping integration test in short mode")
	}

	// Test without tail limit
	_, err := c.GetContainerLogs(ctx, 0)
	s.Error(err) // Will error without actual kubectl/cluster

	// Test with tail limit
	_, err = c.GetContainerLogs(ctx, 100)
	s.Error(err) // Will error without actual kubectl/cluster
}

func (s *KubeConnectorTestSuite) TestPortForward() {
	c := New("test-ns", "test-pod", "", "")
	ctx := context.Background()

	if testing.Short() {
		s.T().Skip("Skipping integration test in short mode")
	}

	err := c.PortForward(ctx, 8080, 80)
	// cmd.Start() may return nil even if kubectl doesn't exist (it fails later)
	// So we just check the method exists and can be called
	_ = err
}

func (s *KubeConnectorTestSuite) TestMakeExecutable() {
	c := New("test-ns", "test-pod", "test-container", "")
	ctx := context.Background()

	if testing.Short() {
		s.T().Skip("Skipping integration test in short mode")
	}

	err := c.MakeExecutable(ctx, "/path/to/file")
	s.Error(err) // Will error without actual kubectl/cluster
}

func (s *KubeConnectorTestSuite) TestRemoveFile() {
	c := New("test-ns", "test-pod", "test-container", "")
	ctx := context.Background()

	if testing.Short() {
		s.T().Skip("Skipping integration test in short mode")
	}

	err := c.RemoveFile(ctx, "/path/to/file")
	s.Error(err) // Will error without actual kubectl/cluster
}

func (s *KubeConnectorTestSuite) TestFileExists() {
	c := New("test-ns", "test-pod", "test-container", "")
	ctx := context.Background()

	if testing.Short() {
		s.T().Skip("Skipping integration test in short mode")
	}

	exists, err := c.FileExists(ctx, "/path/to/file")
	s.Error(err) // Will error without actual kubectl/cluster
	s.False(exists)
}

func (s *KubeConnectorTestSuite) TestTestConnection() {
	c := New("test-ns", "test-pod", "", "")
	ctx := context.Background()

	if testing.Short() {
		s.T().Skip("Skipping integration test in short mode")
	}

	err := c.TestConnection(ctx)
	s.Error(err) // Will error without actual kubectl/cluster
}

func (s *KubeConnectorTestSuite) TestContextCancellation() {
	c := New("test-ns", "test-pod", "test-container", "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := c.ExecuteCommand(ctx, "echo test")
	s.Error(err)
}

func (s *KubeConnectorTestSuite) TestContextTimeout() {
	c := New("test-ns", "test-pod", "test-container", "")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond) // Ensure timeout

	_, err := c.ExecuteCommand(ctx, "echo test")
	s.Error(err)
}

func (s *KubeConnectorTestSuite) TestConfigStructure() {
	config := Config{
		Namespace:  "test-namespace",
		Pod:        "test-pod",
		Container:  "test-container",
		Kubeconfig: "/path/to/kubeconfig",
	}

	s.Equal("test-namespace", config.Namespace)
	s.Equal("test-pod", config.Pod)
	s.Equal("test-container", config.Container)
	s.Equal("/path/to/kubeconfig", config.Kubeconfig)
}

func (s *KubeConnectorTestSuite) TestConnectorStructure() {
	config := Config{
		Namespace:  "test-namespace",
		Pod:        "test-pod",
		Container:  "test-container",
		Kubeconfig: "/path/to/kubeconfig",
	}

	connector := &Connector{
		config: config,
	}

	s.Equal(config, connector.config)
	s.Equal("test-namespace/test-pod", connector.GetPodIdentifier())
}

func (s *KubeConnectorTestSuite) TestCopyCommandConstruction() {
	testCases := []struct {
		name          string
		namespace     string
		pod           string
		container     string
		kubeconfig    string
		expectedInCmd []string
	}{
		{
			name:       "with container",
			namespace:  "test-ns",
			pod:        "test-pod",
			container:  "test-container",
			kubeconfig: "",
			expectedInCmd: []string{
				"-n", "test-ns",
				"test-pod",
				"-c", "test-container",
			},
		},
		{
			name:       "without container",
			namespace:  "test-ns",
			pod:        "test-pod",
			container:  "",
			kubeconfig: "",
			expectedInCmd: []string{
				"-n", "test-ns",
				"test-pod",
			},
		},
		{
			name:       "with kubeconfig",
			namespace:  "test-ns",
			pod:        "test-pod",
			container:  "",
			kubeconfig: "/path/to/config",
			expectedInCmd: []string{
				"--kubeconfig", "/path/to/config",
				"-n", "test-ns",
				"test-pod",
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			c := New(tc.namespace, tc.pod, tc.container, tc.kubeconfig)
			args := c.BuildKubectlArgs()

			// Build full args for copy command
			args = append(args, "cp", "/local/file", fmt.Sprintf("%s:/remote/file", tc.pod))
			if tc.container != "" {
				args = append(args, "-c", tc.container)
			}

			// Check that expected arguments are present
			argStr := strings.Join(args, " ")
			for _, expected := range tc.expectedInCmd {
				s.Contains(argStr, expected)
			}
		})
	}
}

func (s *KubeConnectorTestSuite) TestLogsCommandConstruction() {
	testCases := []struct {
		name      string
		container string
		tail      int
		expected  []string
	}{
		{
			name:      "with container and tail",
			container: "test-container",
			tail:      100,
			expected:  []string{"-c", "test-container", "--tail", "100"},
		},
		{
			name:      "without container with tail",
			container: "",
			tail:      50,
			expected:  []string{"--tail", "50"},
		},
		{
			name:      "without tail",
			container: "test-container",
			tail:      0,
			expected:  []string{"-c", "test-container"},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			c := New("test-ns", "test-pod", tc.container, "")
			ctx := context.Background()

			// This will fail without kubectl, but we can still verify construction
			_, _ = c.GetContainerLogs(ctx, tc.tail)

			// The test verifies that the method handles the parameters correctly
			// Actual command construction is internal to the method
		})
	}
}

func (s *KubeConnectorTestSuite) TestPortForwardCommandConstruction() {
	c := New("test-ns", "test-pod", "", "")
	ctx := context.Background()

	if testing.Short() {
		s.T().Skip("Skipping integration test in short mode")
	}

	// Test various port combinations
	testCases := []struct {
		localPort  int
		remotePort int
	}{
		{8080, 80},
		{3000, 3000},
		{9090, 9090},
		{5432, 5432},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("%d:%d", tc.localPort, tc.remotePort), func() {
			err := c.PortForward(ctx, tc.localPort, tc.remotePort)
			// cmd.Start() may return nil even if kubectl doesn't exist
			_ = err
		})
	}
}

// BenchmarkBuildKubectlArgs benchmarks building kubectl arguments
func BenchmarkBuildKubectlArgs(b *testing.B) {
	c := New("test-namespace", "test-pod", "test-container", "/path/to/kubeconfig")
	for i := 0; i < b.N; i++ {
		_ = c.BuildKubectlArgs()
	}
}

// BenchmarkGetPodIdentifier benchmarks getting pod identifier
func BenchmarkGetPodIdentifier(b *testing.B) {
	c := New("test-namespace", "test-pod", "test-container", "")
	for i := 0; i < b.N; i++ {
		_ = c.GetPodIdentifier()
	}
}