package kube

import (
	"context"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type KubeTestSuite struct {
	suite.Suite
	tool      *Tool
	validator *validator.Validate
}

func (suite *KubeTestSuite) SetupTest() {
	logger := zerolog.Nop()
	suite.validator = validator.New()
	suite.tool = &Tool{
		logger:         logger,
		validator:      suite.validator,
	}
}

func (suite *KubeTestSuite) TestInputValidation() {
	testCases := []struct {
		name        string
		input       Input
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid port-forward action",
			input: Input{
				Action:     "port-forward",
				Namespace:  "default",
				Resource:   "pod/my-pod",
				LocalPort:  8080,
				RemotePort: 8080,
			},
			shouldError: false,
		},
		{
			name: "valid with kubeconfig",
			input: Input{
				Action:     "port-forward",
				Namespace:  "kube-system",
				Resource:   "service/my-service",
				LocalPort:  3000,
				RemotePort: 80,
				KubeConfig: "/path/to/kubeconfig",
				Context:    "my-context",
			},
			shouldError: false,
		},
		{
			name: "invalid action",
			input: Input{
				Action:    "invalid-action",
				Namespace: "default",
				Resource:  "pod/my-pod",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "missing required action",
			input: Input{
				Namespace: "default",
				Resource:  "pod/my-pod",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "invalid namespace with special characters",
			input: Input{
				Action:    "port-forward",
				Namespace: "name@space!",
				Resource:  "pod/my-pod",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "namespace too long",
			input: Input{
				Action:    "port-forward",
				Namespace: string(make([]byte, 64)), // Over 63 limit
				Resource:  "pod/my-pod",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "valid namespace with dashes",
			input: Input{
				Action:    "port-forward",
				Namespace: "kube-system",
				Resource:  "pod/my-pod",
			},
			shouldError: false,
		},
		{
			name: "resource name too long",
			input: Input{
				Action:   "port-forward",
				Resource: string(make([]byte, 513)), // Over 512 limit
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "local port out of range - negative",
			input: Input{
				Action:    "port-forward",
				Resource:  "pod/my-pod",
				LocalPort: -1,
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "local port out of range - too high",
			input: Input{
				Action:    "port-forward",
				Resource:  "pod/my-pod",
				LocalPort: 65536,
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "remote port out of range - negative",
			input: Input{
				Action:     "port-forward",
				Resource:   "pod/my-pod",
				RemotePort: -1,
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "remote port out of range - too high",
			input: Input{
				Action:     "port-forward",
				Resource:   "pod/my-pod",
				RemotePort: 65536,
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "context name too long",
			input: Input{
				Action:   "port-forward",
				Resource: "pod/my-pod",
				Context:  string(make([]byte, 254)), // Over 253 limit
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "valid context with dashes and underscores",
			input: Input{
				Action:   "port-forward",
				Resource: "pod/my-pod",
				Context:  "my-cluster_context",
			},
			shouldError: false,
		},
		{
			name: "invalid context with special characters",
			input: Input{
				Action:   "port-forward",
				Resource: "pod/my-pod",
				Context:  "context@name!",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "empty namespace defaults to default",
			input: Input{
				Action:   "port-forward",
				Resource: "pod/my-pod",
			},
			shouldError: false,
		},
		{
			name: "zero ports use defaults",
			input: Input{
				Action:     "port-forward",
				Resource:   "pod/my-pod",
				LocalPort:  0,
				RemotePort: 0,
			},
			shouldError: false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			err := suite.validator.Struct(tc.input)
			if tc.shouldError {
				suite.Error(err)
				if tc.errorMsg == "validation error" {
					suite.Contains(err.Error(), "validation")
				} else if tc.errorMsg != "" {
					suite.Contains(err.Error(), tc.errorMsg)
				}
			} else {
				suite.NoError(err)
			}
		})
	}
}

func (suite *KubeTestSuite) TestKubeHandlerValidation() {
	ctx := context.Background()
	session := &mcp.ServerSession{}

	testCases := []struct {
		name        string
		input       Input
		shouldError bool
		errorMsg    string
	}{
		{
			name: "invalid action",
			input: Input{
				Action:   "invalid-action",
				Resource: "pod/my-pod",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "missing resource for port-forward",
			input: Input{
				Action: "port-forward",
			},
			shouldError: true,
			errorMsg:    "resource is required",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			params := &mcp.CallToolParamsFor[Input]{
				Arguments: tc.input,
			}
			
			_, err := suite.tool.KubeHandler(ctx, session, params)
			suite.Error(err) // All will error in test environment
			if tc.errorMsg != "" {
				suite.Contains(err.Error(), tc.errorMsg)
			}
		})
	}
}

func (suite *KubeTestSuite) TestPortAvailabilityCheck() {
	ctx := context.Background()
	
	// Test with a known available port (0 should always work for binding test)
	available := suite.tool.isPortAvailable(ctx, 0)
	suite.True(available)
	
	// Test finding available port
	port := suite.tool.findAvailablePort(ctx, 50000) // Use high port range
	suite.Greater(port, 0)
	suite.LessOrEqual(port, 50000+maxPortOffset)
}

func (suite *KubeTestSuite) TestNewCreatesValidTool() {
	logger := zerolog.Nop()
	tool := New(logger)
	
	suite.NotNil(tool)
	kubeTool, ok := tool.(*Tool)
	suite.True(ok)
	suite.NotNil(kubeTool.validator)
	suite.NotNil(kubeTool.logger)
}

func TestKubeTestSuite(t *testing.T) {
	suite.Run(t, new(KubeTestSuite))
}