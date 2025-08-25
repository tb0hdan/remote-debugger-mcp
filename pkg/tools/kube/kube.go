package kube

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
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
)

type Input struct {
	Action       string   `json:"action" validate:"required,oneof=port-forward"` // Action to perform (e.g., "port-forward")
	Namespace    string   `json:"namespace,omitempty" validate:"omitempty,alphanum|contains=-,max=63"` // Kubernetes namespace (default: "default")
	Resource     string   `json:"resource,omitempty" validate:"omitempty,max=512"`     // Resource to act on (e.g., "pod/my-pod", "service/my-service")
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
	default:
		return nil, fmt.Errorf("unsupported action: %s. Supported actions: port-forward", input.Action)
	}
}

func (k *Tool) Register(srv *server.Server) {
	kubeTool := &mcp.Tool{
		Name:        "kube",
		Description: "Kubernetes operations tool for kubectl commands (currently supports port-forward)",
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
		return nil, errors.New("resource is required for port-forward (e.g., 'pod/my-pod', 'service/my-service')")
	}

	// Parse resource to get pod name (format: "pod/my-pod" or just "my-pod")
	resourceParts := strings.Split(input.Resource, "/")
	var podName string
	switch len(resourceParts) {
	case resourcePartsWithType:
		if resourceParts[0] != "pod" {
			return nil, fmt.Errorf("unsupported resource type: %s (only 'pod' supported)", resourceParts[0])
		}
		podName = resourceParts[1]
	case 1:
		// Assume it's just a pod name
		podName = resourceParts[0]
	default:
		return nil, fmt.Errorf("unsupported resource format: %s (only 'pod/name' or 'name' supported)", input.Resource)
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

	// Create kube connector
	connector := kubeconnector.New(input.Namespace, podName, "", input.KubeConfig)

	// Test connection first
	if err := connector.TestConnection(ctx); err != nil {
		// If pod is not running, provide helpful error
		return nil, fmt.Errorf("failed to connect to pod %s in namespace %s: %w", podName, input.Namespace, err)
	}

	// Start port forwarding using the connector
	if err := connector.PortForward(ctx, localPort, remotePort); err != nil {
		return nil, fmt.Errorf("failed to start port forwarding: %w", err)
	}

	// Wait a moment to ensure port forwarding is established
	time.Sleep(startupDelay)

	// Create result
	resultText := "Port forwarding established:\n"
	resultText += fmt.Sprintf("  Namespace: %s\n", input.Namespace)
	resultText += fmt.Sprintf("  Pod: %s\n", podName)
	resultText += fmt.Sprintf("  Local port: %d\n", localPort)
	resultText += fmt.Sprintf("  Remote port: %d\n", remotePort)
	resultText += fmt.Sprintf("\nForwarding is active in background. Access your application at: localhost:%d", localPort)
	
	if input.Context != "" {
		resultText += "\n  Context: " + input.Context
	}

	// Build command string for debugging
	cmdStr := fmt.Sprintf("kubectl port-forward -n %s %s %d:%d", input.Namespace, podName, localPort, remotePort)
	if input.KubeConfig != "" {
		cmdStr = fmt.Sprintf("kubectl --kubeconfig %s port-forward -n %s %s %d:%d", input.KubeConfig, input.Namespace, podName, localPort, remotePort)
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