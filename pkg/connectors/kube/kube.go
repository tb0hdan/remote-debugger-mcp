package kube

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// Config holds Kubernetes connection configuration.
type Config struct {
	Namespace  string
	Pod        string // Can be "pod/name", "deployment/name", or "service/name" for port-forward
	Container  string
	Kubeconfig string
}

// Connector provides Kubernetes connectivity functionality.
type Connector struct {
	config         Config
	portForwardCmd *exec.Cmd
	mu             sync.Mutex
}

// New creates a new Kubernetes connector.
// For port-forward operations, pod parameter can be "pod/name", "deployment/name", or "service/name".
func New(namespace, pod, container, kubeconfig string) *Connector {
	if namespace == "" {
		namespace = "default"
	}
	return &Connector{
		config: Config{
			Namespace:  namespace,
			Pod:        pod,
			Container:  container,
			Kubeconfig: kubeconfig,
		},
		portForwardCmd: nil,
	}
}

// GetPodIdentifier returns the pod identifier string.
func (c *Connector) GetPodIdentifier() string {
	return fmt.Sprintf("%s/%s", c.config.Namespace, c.config.Pod)
}

// BuildKubectlArgs builds common kubectl arguments.
func (c *Connector) BuildKubectlArgs() []string {
	args := []string{}

	// Use provided kubeconfig or default to ~/.kube/config
	kubeconfig := c.config.Kubeconfig
	if kubeconfig == "" {
		if homeDir, err := os.UserHomeDir(); err == nil {
			defaultConfig := filepath.Join(homeDir, ".kube", "config")
			if _, err := os.Stat(defaultConfig); err == nil {
				kubeconfig = defaultConfig
			}
		}
	}

	if kubeconfig != "" {
		args = append(args, "--kubeconfig", kubeconfig)
	}

	if c.config.Namespace != "" {
		args = append(args, "-n", c.config.Namespace)
	}
	return args
}

// ExecuteCommand executes a command in the pod.
// Note: This only works when Pod field contains a pod name, not a resource reference like "deployment/name".
func (c *Connector) ExecuteCommand(ctx context.Context, command string) (string, error) {
	// Extract pod name if it's in format "pod/name"
	podName := strings.TrimPrefix(c.config.Pod, "pod/")

	args := c.BuildKubectlArgs()
	args = append(args, "exec", podName)
	if c.config.Container != "" {
		args = append(args, "-c", c.config.Container)
	}
	args = append(args, "--", "sh", "-c", command)

	cmd := exec.CommandContext(ctx, "kubectl", args...)
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
// Note: This only works when Pod field contains a pod name, not a resource reference like "deployment/name".
func (c *Connector) ExecuteCommandWithExitCode(ctx context.Context, command string) (string, int, error) {
	// Extract pod name if it's in format "pod/name"
	podName := strings.TrimPrefix(c.config.Pod, "pod/")

	args := c.BuildKubectlArgs()
	args = append(args, "exec", podName)
	if c.config.Container != "" {
		args = append(args, "-c", c.config.Container)
	}
	args = append(args, "--", "sh", "-c", command)

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
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

// CopyFileToPod copies a local file to the pod.
// Note: This only works when Pod field contains a pod name, not a resource reference like "deployment/name".
func (c *Connector) CopyFileToPod(ctx context.Context, localPath, remotePath string) error {
	// Extract pod name if it's in format "pod/name"
	podName := strings.TrimPrefix(c.config.Pod, "pod/")

	args := c.BuildKubectlArgs()
	target := fmt.Sprintf("%s:%s", podName, remotePath)
	if c.config.Container != "" {
		target = fmt.Sprintf("%s:%s", podName, remotePath)
		args = append(args, "cp", localPath, target, "-c", c.config.Container)
	} else {
		args = append(args, "cp", localPath, target)
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy file to pod: %v - %s", err, stderr.String())
	}
	return nil
}

// CopyFileFromPod copies a file from the pod to local.
// Note: This only works when Pod field contains a pod name, not a resource reference like "deployment/name".
func (c *Connector) CopyFileFromPod(ctx context.Context, remotePath, localPath string) error {
	// Extract pod name if it's in format "pod/name"
	podName := strings.TrimPrefix(c.config.Pod, "pod/")

	args := c.BuildKubectlArgs()
	source := fmt.Sprintf("%s:%s", podName, remotePath)
	if c.config.Container != "" {
		args = append(args, "cp", source, localPath, "-c", c.config.Container)
	} else {
		args = append(args, "cp", source, localPath)
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy file from pod: %v - %s", err, stderr.String())
	}
	return nil
}

// GetPodStatus gets the status of the pod.
// Note: This only works when Pod field contains a pod name, not a resource reference like "deployment/name".
func (c *Connector) GetPodStatus(ctx context.Context) (string, error) {
	// Extract pod name if it's in format "pod/name"
	podName := strings.TrimPrefix(c.config.Pod, "pod/")

	args := c.BuildKubectlArgs()
	args = append(args, "get", "pod", podName, "-o", "jsonpath={.status.phase}")

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to get pod status: %v - %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// GetContainerLogs gets logs from the container.
// Note: This only works when Pod field contains a pod name, not a resource reference like "deployment/name".
func (c *Connector) GetContainerLogs(ctx context.Context, tail int) (string, error) {
	// Extract pod name if it's in format "pod/name"
	podName := strings.TrimPrefix(c.config.Pod, "pod/")

	args := c.BuildKubectlArgs()
	args = append(args, "logs", podName)
	if c.config.Container != "" {
		args = append(args, "-c", c.config.Container)
	}
	if tail > 0 {
		args = append(args, "--tail", strconv.Itoa(tail))
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to get logs: %v - %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// PortForward sets up port forwarding to the resource (pod, deployment, or service).
func (c *Connector) PortForward(ctx context.Context, localPort, remotePort int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Kill any existing port-forward process
	if err := c.stopPortForward(); err != nil {
		return fmt.Errorf("failed to stop existing port-forward: %w", err)
	}

	args := c.BuildKubectlArgs()
	args = append(args, "port-forward", c.config.Pod,
		fmt.Sprintf("%d:%d", localPort, remotePort))

	// Create a new context that won't be cancelled when the calling context ends
	// This ensures port-forward continues running in the background
	backgroundCtx := context.WithoutCancel(ctx)
	cmd := exec.CommandContext(backgroundCtx, "kubectl", args...)

	// Set process group ID so we can kill the entire group
	// Also ensure the process is detached from the parent process
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	// Redirect stdout/stderr to prevent hanging on pipe operations
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start port-forward: %w", err)
	}

	c.portForwardCmd = cmd

	// Start a goroutine to reap the process when it exits.
	go func() {
		_ = cmd.Wait()
	}()

	return nil
}

// StopPortForward stops any active port forwarding process.
func (c *Connector) StopPortForward() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stopPortForward()
}

// MakeExecutable makes a file executable in the pod.
func (c *Connector) MakeExecutable(ctx context.Context, remotePath string) error {
	_, err := c.ExecuteCommand(ctx, "chmod +x "+remotePath)
	return err
}

// RemoveFile removes a file in the pod.
func (c *Connector) RemoveFile(ctx context.Context, remotePath string) error {
	_, err := c.ExecuteCommand(ctx, "rm -f "+remotePath)
	return err
}

// FileExists checks if a file exists in the pod.
func (c *Connector) FileExists(ctx context.Context, remotePath string) (bool, error) {
	output, err := c.ExecuteCommand(ctx, fmt.Sprintf("test -f %s && echo 'exists' || echo 'not exists'", remotePath))
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) == "exists", nil
}

// TestConnection tests the connection to the pod.
// For deployments and services, this method may not work as expected - use checkResourceExists in the kube tool instead.
func (c *Connector) TestConnection(ctx context.Context) error {
	// If Pod contains "/" it's a resource reference (deployment/name or service/name)
	// For those, we can't check pod status directly
	if strings.Contains(c.config.Pod, "/") {
		// For non-pod resources, we can't test the connection this way
		// The kube tool should handle verification differently
		return nil
	}

	status, err := c.GetPodStatus(ctx)
	if err != nil {
		return err
	}
	if status != "Running" {
		return fmt.Errorf("pod is not running, status: %s", status)
	}
	return nil
}

// stopPortForward stops the port forwarding process (internal method).
// Must be called with mutex held.
func (c *Connector) stopPortForward() error {
	if c.portForwardCmd == nil || c.portForwardCmd.Process == nil {
		return nil
	}

	// Kill the process group to ensure all child processes are terminated
	pgid := c.portForwardCmd.Process.Pid
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		// If SIGTERM fails, try SIGKILL
		if killErr := syscall.Kill(-pgid, syscall.SIGKILL); killErr != nil {
			// Log the error but don't fail if the process is already gone
			if !strings.Contains(killErr.Error(), "no such process") {
				return fmt.Errorf("failed to kill port-forward process: %v", killErr)
			}
		}
	}

	// We set to nil here to indicate that the process is being terminated.
	if c.portForwardCmd != nil {
		_ = c.portForwardCmd.Wait()
	}
	c.portForwardCmd = nil

	return nil
}
