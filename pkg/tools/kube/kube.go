package kube

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
	kubeconnector "github.com/tb0hdan/remote-debugger-mcp/pkg/connectors/kube"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/server"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/tools"
)

const (
	defaultLocalPort      = 6060
	maxPortRange          = 99
	maxPortOffset         = 100
	startupDelay          = 500 * time.Millisecond
	resourcePartsWithType = 2 // Format: "type/name" e.g., "pod/my-pod"
	labelSelectorParts    = 2 // Format: "key:value" for label selector parsing
)

type Input struct {
	Action       string   `json:"action" validate:"required,oneof=port-forward stop-port-forward get create delete apply"` // Action to perform (e.g., "port-forward", "stop-port-forward", "get", "create", "delete", "apply")
	Namespace    string   `json:"namespace,omitempty" validate:"omitempty,alphanum|contains=-,max=63"` // Kubernetes namespace (default: "default")
	Resource     string   `json:"resource,omitempty" validate:"omitempty,max=512"`     // Resource to act on (e.g., "pod/my-pod", "deployment/my-deployment", "service/my-service")
	LocalPort    int      `json:"local_port,omitempty" validate:"min=0,max=65535"`   // Local port for port-forward (default: 6060)
	RemotePort   int      `json:"remote_port,omitempty" validate:"min=0,max=65535"`  // Remote port for port-forward
	KubeConfig   string   `json:"kubeconfig,omitempty" validate:"omitempty,filepath"`   // Path to kubeconfig file
	Context      string   `json:"context,omitempty" validate:"omitempty,alphanum|contains=-|contains=_,max=253"`      // Kubernetes context to use
	ExtraArgs    []string `json:"extra_args,omitempty"`   // Additional kubectl arguments
}

type Output struct {
	Action     string `json:"action"`
	Namespace  string `json:"namespace"`
	Resource   string `json:"resource,omitempty"`
	LocalPort  int    `json:"local_port,omitempty"`
	RemotePort int    `json:"remote_port,omitempty"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	Command    string `json:"command,omitempty"`
}

type Tool struct {
	logger         zerolog.Logger
	validator      *validator.Validate
	lastConnector  *kubeconnector.Connector
	mu             sync.Mutex
}

func (k *Tool) KubeHandler(ctx context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[Input]) (*mcp.CallToolResultFor[Output], error) {
	input := params.Arguments

	// Validate input using validator
	if err := k.validator.Struct(input); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// Set defaults
	if input.Namespace == "" {
		input.Namespace = "default"
	}

	switch input.Action {
	case "port-forward":
		return k.handlePortForward(ctx, &input)
	case "stop-port-forward":
		return k.handleStopPortForward(ctx, &input)
	case "get", "create", "delete", "apply":
		return k.handleKubectlCommand(ctx, &input)
	default:
		return nil, fmt.Errorf("unsupported action: %s. Supported actions: port-forward, stop-port-forward, get, create, delete, apply", input.Action)
	}
}

func (k *Tool) Register(srv *server.Server) {
	kubeTool := &mcp.Tool{
		Name:        "kube",
		Description: "Kubernetes operations tool for kubectl commands (supports port-forward for pods, deployments, and services with proper cleanup)",
	}

	mcp.AddTool(&srv.Server, kubeTool, k.KubeHandler)
	k.logger.Debug().Msg("kube tool registered")
}

func New(logger zerolog.Logger) tools.Tool {
	validate := validator.New()

	return &Tool{
		logger:         logger.With().Str("tool", "kube").Logger(),
		validator:      validate,
	}
}

// isPortAvailable checks if a local port is available for binding.
func (k *Tool) isPortAvailable(ctx context.Context, port int) bool {
	lc := net.ListenConfig{}
	listener, err := lc.Listen(ctx, "tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}

// findAvailablePort finds an available port starting from the given port.
func (k *Tool) findAvailablePort(ctx context.Context, startPort int) int {
	for port := startPort; port < startPort+maxPortOffset; port++ {
		if k.isPortAvailable(ctx, port) {
			return port
		}
	}
	return 0
}

func (k *Tool) handlePortForward(ctx context.Context, input *Input) (*mcp.CallToolResultFor[Output], error) {
	// Validate input
	if input.Resource == "" {
		return nil, errors.New("resource is required for port-forward (e.g., 'pod/my-pod', 'deployment/my-deployment', 'service/my-service')")
	}

	// Parse resource to get type and name (format: "type/name" or just "name")
	resource := input.Resource
	resourceParts := strings.Split(input.Resource, "/")
	var resourceType, resourceName string
	
	switch len(resourceParts) {
	case resourcePartsWithType:
		resourceType = resourceParts[0]
		resourceName = resourceParts[1]
	case 1:
		// Assume it's just a pod name
		resourceType = "pod"
		resourceName = resourceParts[0]
		resource = "pod/" + resourceName
	default:
		return nil, fmt.Errorf("unsupported resource format: %s (expected 'type/name' or 'name')", input.Resource)
	}

	// Normalize resource type aliases
	switch resourceType {
	case "deploy":
		resourceType = "deployment"
		resource = "deployment/" + resourceName
	case "svc":
		resourceType = "service"
		resource = "service/" + resourceName
	case "pod", "deployment", "service":
		// Already valid
	default:
		return nil, fmt.Errorf("unsupported resource type: %s (supported: pod, deployment, service)", resourceType)
	}

	// Set default local port
	localPort := defaultLocalPort
	if input.LocalPort > 0 {
		localPort = input.LocalPort
	}

	// Check if port is available
	if !k.isPortAvailable(ctx, localPort) {
		// Try to find an available port
		availablePort := k.findAvailablePort(ctx, localPort)
		if availablePort == 0 {
			return nil, fmt.Errorf("port %d is not available and no alternative ports found in range %d-%d", localPort, localPort, localPort+maxPortRange)
		}
		k.logger.Info().Msgf("Port %d is not available, using port %d instead", localPort, availablePort)
		localPort = availablePort
	}

	// If remote port not specified, use the same as local port
	remotePort := input.RemotePort
	if remotePort == 0 {
		remotePort = localPort
	}

	// Create kube connector with the full resource specification
	connector := kubeconnector.New(input.Namespace, resource, "", input.KubeConfig)
	
	// Store the connector for cleanup
	k.mu.Lock()
	k.lastConnector = connector
	k.mu.Unlock()

	// Test connection first (for pods) or check resource exists (for deployments/services)
	if resourceType == "pod" {
		if err := connector.TestConnection(ctx); err != nil {
			return nil, fmt.Errorf("failed to connect to pod %s in namespace %s: %w", resourceName, input.Namespace, err)
		}
	} else {
		// For deployments and services, check if the resource exists
		if err := k.checkResourceExists(ctx, input.Namespace, resourceType, resourceName, input.KubeConfig, input.Context); err != nil {
			return nil, fmt.Errorf("failed to verify %s %s in namespace %s: %w", resourceType, resourceName, input.Namespace, err)
		}
	}

	// Start port forwarding using the connector (works with pods, deployments, and services)
	if err := connector.PortForward(ctx, localPort, remotePort); err != nil {
		return nil, fmt.Errorf("failed to start port forwarding: %w", err)
	}

	// Wait a moment to ensure port forwarding is established
	time.Sleep(startupDelay)

	// Create result
	resultText := "Port forwarding established:\n"
	resultText += fmt.Sprintf("  Namespace: %s\n", input.Namespace)
	resultText += fmt.Sprintf("  Resource: %s\n", resource)
	resultText += fmt.Sprintf("  Local port: %d\n", localPort)
	resultText += fmt.Sprintf("  Remote port: %d\n", remotePort)
	resultText += fmt.Sprintf("\nForwarding is active in background. Access your application at: localhost:%d", localPort)
	
	if input.Context != "" {
		resultText += "\n  Context: " + input.Context
	}

	// Build command string for debugging
	cmdStr := fmt.Sprintf("kubectl port-forward -n %s %s %d:%d", input.Namespace, resource, localPort, remotePort)
	if input.KubeConfig != "" {
		cmdStr = fmt.Sprintf("kubectl --kubeconfig %s port-forward -n %s %s %d:%d", input.KubeConfig, input.Namespace, resource, localPort, remotePort)
	}
	k.logger.Debug().Msgf("Executed: %s", cmdStr)

	result := &mcp.CallToolResultFor[Output]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: resultText,
			},
		},
	}

	return result, nil
}

// checkResourceExists verifies that a Kubernetes resource exists.
func (k *Tool) checkResourceExists(ctx context.Context, namespace, resourceType, resourceName, kubeconfig, context string) error {
	args := []string{}
	
	// Use provided kubeconfig or default to ~/.kube/config
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
	if context != "" {
		args = append(args, "--context", context)
	}
	args = append(args, "-n", namespace, "get", resourceType, resourceName)

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s not found: %v - %s", resourceType, err, stderr.String())
	}
	
	k.logger.Debug().Msgf("Verified %s %s exists in namespace %s", resourceType, resourceName, namespace)
	return nil
}

// handleStopPortForward stops any active port forwarding.
func (k *Tool) handleStopPortForward(_ context.Context, _ *Input) (*mcp.CallToolResultFor[Output], error) {
	k.mu.Lock()
	connector := k.lastConnector
	k.mu.Unlock()
	
	if connector == nil {
		return &mcp.CallToolResultFor[Output]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: "No active port forwarding found",
				},
			},
		}, nil
	}
	
	if err := connector.StopPortForward(); err != nil {
		return nil, fmt.Errorf("failed to stop port forwarding: %w", err)
	}
	
	k.mu.Lock()
	k.lastConnector = nil
	k.mu.Unlock()
	
	return &mcp.CallToolResultFor[Output]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: "Port forwarding stopped successfully",
			},
		},
	}, nil
}

// handleKubectlCommand handles generic kubectl commands (get, create, delete, apply).
func (k *Tool) handleKubectlCommand(ctx context.Context, input *Input) (*mcp.CallToolResultFor[Output], error) {
	args := []string{}
	
	// Use provided kubeconfig or default to ~/.kube/config
	kubeconfig := input.KubeConfig
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
	if input.Context != "" {
		args = append(args, "--context", input.Context)
	}
	if input.Namespace != "" {
		args = append(args, "-n", input.Namespace)
	}
	
	// Add the action (get, create, delete, apply)
	args = append(args, input.Action)
	
	// Add resource if specified
	if input.Resource != "" {
		args = append(args, input.Resource)
	}
	
	// Add extra arguments
	args = append(args, input.ExtraArgs...)

	// Build command string for output
	cmdStr := "kubectl " + strings.Join(args, " ")

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	
	// Prepare result text
	resultText := fmt.Sprintf("Command: %s\n\n", cmdStr)
	
	if stdout.Len() > 0 {
		resultText += "Output:\n" + stdout.String()
	}
	
	if stderr.Len() > 0 {
		if stdout.Len() > 0 {
			resultText += "\n"
		}
		resultText += "Errors:\n" + stderr.String()
	}
	
	if err != nil {
		resultText += fmt.Sprintf("\nCommand failed with error: %v", err)
	}

	result := &mcp.CallToolResultFor[Output]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: resultText,
			},
		},
	}

	return result, nil
}