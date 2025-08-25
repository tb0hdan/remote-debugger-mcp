package kube

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Config holds Kubernetes connection configuration.
type Config struct {
	Namespace  string
	Pod        string
	Container  string
	Kubeconfig string
}

// Connector provides Kubernetes connectivity functionality.
type Connector struct {
	config Config
}

// New creates a new Kubernetes connector.
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
	}
}

// GetPodIdentifier returns the pod identifier string.
func (c *Connector) GetPodIdentifier() string {
	return fmt.Sprintf("%s/%s", c.config.Namespace, c.config.Pod)
}

// BuildKubectlArgs builds common kubectl arguments.
func (c *Connector) BuildKubectlArgs() []string {
	args := []string{}
	if c.config.Kubeconfig != "" {
		args = append(args, "--kubeconfig", c.config.Kubeconfig)
	}
	if c.config.Namespace != "" {
		args = append(args, "-n", c.config.Namespace)
	}
	return args
}

// ExecuteCommand executes a command in the pod.
func (c *Connector) ExecuteCommand(ctx context.Context, command string) (string, error) {
	args := c.BuildKubectlArgs()
	args = append(args, "exec", c.config.Pod)
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
func (c *Connector) ExecuteCommandWithExitCode(ctx context.Context, command string) (string, int, error) {
	args := c.BuildKubectlArgs()
	args = append(args, "exec", c.config.Pod)
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
func (c *Connector) CopyFileToPod(ctx context.Context, localPath, remotePath string) error {
	args := c.BuildKubectlArgs()
	target := fmt.Sprintf("%s:%s", c.config.Pod, remotePath)
	if c.config.Container != "" {
		target = fmt.Sprintf("%s:%s", c.config.Pod, remotePath)
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
func (c *Connector) CopyFileFromPod(ctx context.Context, remotePath, localPath string) error {
	args := c.BuildKubectlArgs()
	source := fmt.Sprintf("%s:%s", c.config.Pod, remotePath)
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
func (c *Connector) GetPodStatus(ctx context.Context) (string, error) {
	args := c.BuildKubectlArgs()
	args = append(args, "get", "pod", c.config.Pod, "-o", "jsonpath={.status.phase}")

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
func (c *Connector) GetContainerLogs(ctx context.Context, tail int) (string, error) {
	args := c.BuildKubectlArgs()
	args = append(args, "logs", c.config.Pod)
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

// PortForward sets up port forwarding to the pod.
func (c *Connector) PortForward(ctx context.Context, localPort, remotePort int) error {
	args := c.BuildKubectlArgs()
	args = append(args, "port-forward", c.config.Pod,
		fmt.Sprintf("%d:%d", localPort, remotePort))

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	return cmd.Start()
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
func (c *Connector) TestConnection(ctx context.Context) error {
	status, err := c.GetPodStatus(ctx)
	if err != nil {
		return err
	}
	if status != "Running" {
		return fmt.Errorf("pod is not running, status: %s", status)
	}
	return nil
}