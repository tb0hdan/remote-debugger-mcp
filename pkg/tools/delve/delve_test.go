package delve

import (
	"context"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type DelveTestSuite struct {
	suite.Suite
	tool      *Tool
	validator *validator.Validate
}

func (suite *DelveTestSuite) SetupTest() {
	logger := zerolog.Nop()
	suite.validator = validator.New()
	suite.tool = &Tool{
		logger:    logger,
		validator: suite.validator,
	}
}

func (suite *DelveTestSuite) TestInputValidation() {
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
				Port:    2345,
				Command: "help",
			},
			shouldError: false,
		},
		{
			name: "valid IP address",
			input: Input{
				Host:    "192.168.1.1",
				Port:    2345,
				Command: "break main.go:10",
			},
			shouldError: false,
		},
		{
			name: "invalid hostname with special characters",
			input: Input{
				Host:    "host$name!",
				Port:    2345,
				Command: "help",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "port out of range - negative",
			input: Input{
				Host:    "localhost",
				Port:    -1,
				Command: "help",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "port out of range - too high",
			input: Input{
				Host:    "localhost",
				Port:    65536,
				Command: "help",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "command too long",
			input: Input{
				Host:    "localhost",
				Port:    2345,
				Command: string(make([]byte, 4097)), // Over 4096 limit
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "negative offset",
			input: Input{
				Host:    "localhost",
				Port:    2345,
				Command: "help",
				Offset:  -1,
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "negative max lines",
			input: Input{
				Host:     "localhost",
				Port:     2345,
				Command:  "help",
				MaxLines: -1,
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "max lines exceeds limit",
			input: Input{
				Host:     "localhost",
				Port:     2345,
				Command:  "help",
				MaxLines: 100001,
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "empty host defaults to localhost",
			input: Input{
				Port:    2345,
				Command: "help",
			},
			shouldError: false,
		},
		{
			name: "zero port should use default",
			input: Input{
				Host:    "localhost",
				Port:    0,
				Command: "help",
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

func (suite *DelveTestSuite) TestDelveHandlerValidation() {
	ctx := context.Background()
	session := &mcp.ServerSession{}

	testCases := []struct {
		name        string
		input       Input
		shouldError bool
	}{
		{
			name: "invalid host",
			input: Input{
				Host: "invalid@host",
				Port: 2345,
			},
			shouldError: true,
		},
		{
			name: "invalid port",
			input: Input{
				Host: "localhost",
				Port: 70000,
			},
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			params := &mcp.CallToolParamsFor[Input]{
				Arguments: tc.input,
			}
			
			_, err := suite.tool.DelveHandler(ctx, session, params)
			if tc.shouldError {
				suite.Error(err)
				suite.Contains(err.Error(), "validation error")
			} else {
				// Note: This will fail to connect, but validation should pass
				suite.Error(err)
				suite.NotContains(err.Error(), "validation error")
			}
		})
	}
}

func (suite *DelveTestSuite) TestNewCreatesValidTool() {
	logger := zerolog.Nop()
	tool := New(logger)
	
	suite.NotNil(tool)
	delveTool, ok := tool.(*Tool)
	suite.True(ok)
	suite.NotNil(delveTool.validator)
	suite.NotNil(delveTool.logger)
}

func TestDelveTestSuite(t *testing.T) {
	suite.Run(t, new(DelveTestSuite))
}