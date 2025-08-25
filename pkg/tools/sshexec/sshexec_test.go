package sshexec

import (
	"context"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type SSHExecTestSuite struct {
	suite.Suite
	tool      *Tool
	validator *validator.Validate
}

func (suite *SSHExecTestSuite) SetupTest() {
	logger := zerolog.Nop()
	suite.validator = validator.New()
	suite.tool = &Tool{
		logger:    logger,
		validator: suite.validator,
	}
}

func (suite *SSHExecTestSuite) TestInputValidation() {
	testCases := []struct {
		name        string
		input       Input
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid execution mode",
			input: Input{
				Host:       "192.168.1.1",
				Port:       22,
				User:       "testuser",
				BinaryPath: "/usr/bin/test",
				RemotePath: "/tmp/test",
			},
			shouldError: false,
		},
		{
			name: "valid kill by PID",
			input: Input{
				Host:    "example.com",
				Port:    22,
				User:    "admin",
				KillPID: 1234,
			},
			shouldError: false,
		},
		{
			name: "valid kill by name",
			input: Input{
				Host:       "localhost",
				Port:       22,
				User:       "user",
				KillByName: "process_name",
			},
			shouldError: false,
		},
		{
			name: "invalid hostname with spaces",
			input: Input{
				Host:       "host name",
				Port:       22,
				BinaryPath: "/usr/bin/test",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "invalid hostname with special chars",
			input: Input{
				Host:       "host@name!",
				Port:       22,
				BinaryPath: "/usr/bin/test",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "port out of range - negative",
			input: Input{
				Host:       "localhost",
				Port:       -1,
				BinaryPath: "/usr/bin/test",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "port out of range - too high",
			input: Input{
				Host:       "localhost",
				Port:       65536,
				BinaryPath: "/usr/bin/test",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "invalid username with special chars",
			input: Input{
				Host:       "localhost",
				Port:       22,
				User:       "user@name!",
				BinaryPath: "/usr/bin/test",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "username too long",
			input: Input{
				Host:       "localhost",
				Port:       22,
				User:       "verylongusernamethatexceedsthemaximumlength",
				BinaryPath: "/usr/bin/test",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "valid username with dash and underscore",
			input: Input{
				Host:       "localhost",
				Port:       22,
				User:       "test-user_01",
				BinaryPath: "/usr/bin/test",
			},
			shouldError: false,
		},
		{
			name: "remote path too long",
			input: Input{
				Host:       "localhost",
				Port:       22,
				BinaryPath: "/usr/bin/test",
				RemotePath: string(make([]byte, 4097)),
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "negative PID",
			input: Input{
				Host:    "localhost",
				Port:    22,
				KillPID: -1,
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "PID exceeds max int32",
			input: Input{
				Host:    "localhost",
				Port:    22,
				KillPID: 2147483648,
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "kill by name too long",
			input: Input{
				Host:       "localhost",
				Port:       22,
				KillByName: string(make([]byte, 256)),
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "invalid kill signal",
			input: Input{
				Host:       "localhost",
				Port:       22,
				KillPID:    1234,
				KillSignal: "INVALID-123",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "kill signal too long",
			input: Input{
				Host:       "localhost",
				Port:       22,
				KillPID:    1234,
				KillSignal: "VERYLONGSIGNALNAME",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "valid kill signals",
			input: Input{
				Host:       "localhost",
				Port:       22,
				KillPID:    1234,
				KillSignal: "TERM",
			},
			shouldError: false,
		},
		{
			name: "negative max lines",
			input: Input{
				Host:       "localhost",
				Port:       22,
				BinaryPath: "/usr/bin/test",
				MaxLines:   -1,
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "max lines exceeds limit",
			input: Input{
				Host:       "localhost",
				Port:       22,
				BinaryPath: "/usr/bin/test",
				MaxLines:   100001,
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "negative offset",
			input: Input{
				Host:       "localhost",
				Port:       22,
				BinaryPath: "/usr/bin/test",
				Offset:     -1,
			},
			shouldError: true,
			errorMsg:    "validation error",
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

func (suite *SSHExecTestSuite) TestSSHExecHandlerValidation() {
	ctx := context.Background()
	session := &mcp.ServerSession{}

	testCases := []struct {
		name        string
		input       Input
		shouldError bool
		errorMsg    string
	}{
		{
			name: "missing host",
			input: Input{
				Port:       22,
				BinaryPath: "/usr/bin/test",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "invalid host format",
			input: Input{
				Host:       "invalid@host",
				Port:       22,
				BinaryPath: "/usr/bin/test",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "both kill and exec specified",
			input: Input{
				Host:       "localhost",
				Port:       22,
				BinaryPath: "/usr/bin/test",
				KillPID:    1234,
			},
			shouldError: true,
			errorMsg:    "cannot specify both kill parameters and binary_path",
		},
		{
			name: "both kill PID and kill by name",
			input: Input{
				Host:       "localhost",
				Port:       22,
				KillPID:    1234,
				KillByName: "process",
			},
			shouldError: true,
			errorMsg:    "cannot specify both kill_pid and kill_by_name",
		},
		{
			name: "no operation specified",
			input: Input{
				Host: "localhost",
				Port: 22,
			},
			shouldError: true,
			errorMsg:    "must specify either kill parameters",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			params := &mcp.CallToolParamsFor[Input]{
				Arguments: tc.input,
			}
			
			_, err := suite.tool.SSHExecHandler(ctx, session, params)
			suite.Error(err)
			if tc.errorMsg != "" {
				suite.Contains(err.Error(), tc.errorMsg)
			}
		})
	}
}

func (suite *SSHExecTestSuite) TestNewCreatesValidTool() {
	logger := zerolog.Nop()
	tool := New(logger)
	
	suite.NotNil(tool)
	sshTool, ok := tool.(*Tool)
	suite.True(ok)
	suite.NotNil(sshTool.validator)
	suite.NotNil(sshTool.logger)
}

func TestSSHExecTestSuite(t *testing.T) {
	suite.Run(t, new(SSHExecTestSuite))
}