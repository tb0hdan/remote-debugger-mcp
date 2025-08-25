package pprof

import (
	"context"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type PprofTestSuite struct {
	suite.Suite
	tool      *Tool
	validator *validator.Validate
}

func (suite *PprofTestSuite) SetupTest() {
	logger := zerolog.Nop()
	suite.validator = validator.New()
	suite.tool = &Tool{
		logger:    logger,
		validator: suite.validator,
	}
}

func (suite *PprofTestSuite) TestInputValidation() {
	testCases := []struct {
		name        string
		input       Input
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid localhost with default port",
			input: Input{
				Host:    "localhost",
				Port:    6060,
				Profile: "heap",
				Seconds: 30,
			},
			shouldError: false,
		},
		{
			name: "valid IP address",
			input: Input{
				Host:    "192.168.1.1",
				Port:    6060,
				Profile: "profile",
				Seconds: 60,
			},
			shouldError: false,
		},
		{
			name: "valid profile with path",
			input: Input{
				Host:    "localhost",
				Port:    6060,
				Profile: "debug/pprof/heap",
				Seconds: 30,
			},
			shouldError: false,
		},
		{
			name: "invalid hostname with special characters",
			input: Input{
				Host:    "host$name!",
				Port:    6060,
				Profile: "heap",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "port out of range - negative",
			input: Input{
				Host:    "localhost",
				Port:    -1,
				Profile: "heap",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "port out of range - too high",
			input: Input{
				Host:    "localhost",
				Port:    65536,
				Profile: "heap",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "profile name too long",
			input: Input{
				Host:    "localhost",
				Port:    6060,
				Profile: string(make([]byte, 256)),
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "invalid profile with special characters",
			input: Input{
				Host:    "localhost",
				Port:    6060,
				Profile: "heap@profile!",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "negative seconds",
			input: Input{
				Host:    "localhost",
				Port:    6060,
				Profile: "profile",
				Seconds: -1,
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "seconds exceeds limit (1 hour)",
			input: Input{
				Host:    "localhost",
				Port:    6060,
				Profile: "profile",
				Seconds: 3601,
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "negative max lines",
			input: Input{
				Host:     "localhost",
				Port:     6060,
				Profile:  "heap",
				MaxLines: -1,
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "max lines exceeds limit",
			input: Input{
				Host:     "localhost",
				Port:     6060,
				Profile:  "heap",
				MaxLines: 100001,
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "negative offset",
			input: Input{
				Host:    "localhost",
				Port:    6060,
				Profile: "heap",
				Offset:  -1,
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "empty host defaults to localhost",
			input: Input{
				Port:    6060,
				Profile: "heap",
			},
			shouldError: false,
		},
		{
			name: "zero port uses default",
			input: Input{
				Host:    "localhost",
				Port:    0,
				Profile: "heap",
			},
			shouldError: false,
		},
		{
			name: "empty profile lists available profiles",
			input: Input{
				Host: "localhost",
				Port: 6060,
			},
			shouldError: false,
		},
		{
			name: "list profile",
			input: Input{
				Host:    "localhost",
				Port:    6060,
				Profile: "list",
			},
			shouldError: false,
		},
		{
			name: "valid profile names",
			input: Input{
				Host:    "localhost",
				Port:    6060,
				Profile: "goroutine",
			},
			shouldError: false,
		},
		{
			name: "valid seconds at boundary",
			input: Input{
				Host:    "localhost",
				Port:    6060,
				Profile: "profile",
				Seconds: 3600, // exactly 1 hour
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

func (suite *PprofTestSuite) TestPprofHandlerValidation() {
	ctx := context.Background()
	session := &mcp.ServerSession{}

	testCases := []struct {
		name        string
		input       Input
		shouldError bool
		errorMsg    string
	}{
		{
			name: "invalid host",
			input: Input{
				Host:    "invalid@host",
				Port:    6060,
				Profile: "heap",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "invalid port",
			input: Input{
				Host:    "localhost",
				Port:    70000,
				Profile: "heap",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "invalid profile with special chars",
			input: Input{
				Host:    "localhost",
				Port:    6060,
				Profile: "heap@profile!",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "list profiles - valid request",
			input: Input{
				Host:    "localhost",
				Port:    6060,
				Profile: "list",
			},
			shouldError: true, // Will fail to connect, but validation passes
			errorMsg:    "", // Should not contain validation error
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			params := &mcp.CallToolParamsFor[Input]{
				Arguments: tc.input,
			}
			
			_, err := suite.tool.PprofHandler(ctx, session, params)
			suite.Error(err) // All will error due to no server running
			if tc.errorMsg != "" && tc.shouldError {
				suite.Contains(err.Error(), tc.errorMsg)
			} else if tc.errorMsg == "" && tc.shouldError {
				// Should fail for other reasons (connection), not validation
				suite.NotContains(err.Error(), "validation error")
			}
		})
	}
}

func (suite *PprofTestSuite) TestNewCreatesValidTool() {
	logger := zerolog.Nop()
	tool := New(logger)
	
	suite.NotNil(tool)
	pprofTool, ok := tool.(*Tool)
	suite.True(ok)
	suite.NotNil(pprofTool.validator)
	suite.NotNil(pprofTool.logger)
}

func TestPprofTestSuite(t *testing.T) {
	suite.Run(t, new(PprofTestSuite))
}