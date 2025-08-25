package sysinfo

import (
	"context"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type SysinfoTestSuite struct {
	suite.Suite
	tool      *Tool
	validator *validator.Validate
}

func (suite *SysinfoTestSuite) SetupTest() {
	logger := zerolog.Nop()
	suite.validator = validator.New()
	suite.tool = &Tool{
		logger:    logger,
		validator: suite.validator,
	}
}

func (suite *SysinfoTestSuite) TestInputValidation() {
	testCases := []struct {
		name        string
		input       Input
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid local execution (no SSH)",
			input: Input{
				MaxLines: 1000,
				Offset:   0,
			},
			shouldError: false,
		},
		{
			name: "valid remote execution with SSH",
			input: Input{
				SSHHost:  "192.168.1.1",
				SSHPort:  22,
				SSHUser:  "testuser",
				MaxLines: 500,
				Offset:   10,
			},
			shouldError: false,
		},
		{
			name: "valid hostname",
			input: Input{
				SSHHost: "example.com",
				SSHPort: 22,
				SSHUser: "admin",
			},
			shouldError: false,
		},
		{
			name: "invalid SSH host with special characters",
			input: Input{
				SSHHost: "host$name!",
				SSHPort: 22,
				SSHUser: "user",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "invalid SSH host with spaces",
			input: Input{
				SSHHost: "host name",
				SSHPort: 22,
				SSHUser: "user",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "SSH port out of range - negative",
			input: Input{
				SSHHost: "localhost",
				SSHPort: -1,
				SSHUser: "user",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "SSH port out of range - too high",
			input: Input{
				SSHHost: "localhost",
				SSHPort: 65536,
				SSHUser: "user",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "valid SSH port at boundaries",
			input: Input{
				SSHHost: "localhost",
				SSHPort: 65535, // Max valid port
				SSHUser: "user",
			},
			shouldError: false,
		},
		{
			name: "invalid SSH user with special characters",
			input: Input{
				SSHHost: "localhost",
				SSHPort: 22,
				SSHUser: "user@name!",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "SSH user too long",
			input: Input{
				SSHHost: "localhost",
				SSHPort: 22,
				SSHUser: "verylongusernamethatexceedsthemaximumlength",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "valid SSH user with dash and underscore",
			input: Input{
				SSHHost: "localhost",
				SSHPort: 22,
				SSHUser: "test-user_01",
			},
			shouldError: false,
		},
		{
			name: "negative max lines",
			input: Input{
				SSHHost:  "localhost",
				SSHPort:  22,
				SSHUser:  "user",
				MaxLines: -1,
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "max lines exceeds limit",
			input: Input{
				SSHHost:  "localhost",
				SSHPort:  22,
				SSHUser:  "user",
				MaxLines: 100001,
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "max lines at boundary",
			input: Input{
				SSHHost:  "localhost",
				SSHPort:  22,
				SSHUser:  "user",
				MaxLines: 100000, // Exactly at limit
			},
			shouldError: false,
		},
		{
			name: "negative offset",
			input: Input{
				SSHHost: "localhost",
				SSHPort: 22,
				SSHUser: "user",
				Offset:  -1,
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "zero offset is valid",
			input: Input{
				SSHHost: "localhost",
				SSHPort: 22,
				SSHUser: "user",
				Offset:  0,
			},
			shouldError: false,
		},
		{
			name: "empty SSH fields for local execution",
			input: Input{
				MaxLines: 100,
				Offset:   0,
			},
			shouldError: false,
		},
		{
			name: "partial SSH config - only host",
			input: Input{
				SSHHost: "localhost",
			},
			shouldError: false, // SSH port and user are optional
		},
		{
			name: "default values",
			input: Input{}, // All default values
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

func (suite *SysinfoTestSuite) TestSysInfoHandlerValidation() {
	ctx := context.Background()
	session := &mcp.ServerSession{}

	testCases := []struct {
		name        string
		input       Input
		shouldError bool
		errorMsg    string
	}{
		{
			name: "invalid SSH host",
			input: Input{
				SSHHost: "invalid@host",
				SSHPort: 22,
				SSHUser: "user",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "invalid SSH port",
			input: Input{
				SSHHost: "localhost",
				SSHPort: 70000,
				SSHUser: "user",
			},
			shouldError: true,
			errorMsg:    "validation error",
		},
		{
			name: "valid local execution",
			input: Input{
				MaxLines: 100,
			},
			shouldError: false, // Should work for local system
		},
		{
			name: "remote execution will fail connection",
			input: Input{
				SSHHost: "nonexistent.host.local",
				SSHPort: 22,
				SSHUser: "user",
			},
			shouldError: true, // Will fail to connect
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			params := &mcp.CallToolParamsFor[Input]{
				Arguments: tc.input,
			}
			
			result, err := suite.tool.SysInfoHandler(ctx, session, params)
			if tc.shouldError {
				if tc.errorMsg != "" {
					suite.Error(err)
					suite.Contains(err.Error(), tc.errorMsg)
				} else {
					suite.Error(err)
				}
			} else {
				// Local execution should work
				suite.NoError(err)
				suite.NotNil(result)
				suite.Len(result.Content, 1)
			}
		})
	}
}

func (suite *SysinfoTestSuite) TestCalculateCPUUsage() {
	testCases := []struct {
		name     string
		stat1    string
		stat2    string
		expected float64
	}{
		{
			name:     "normal CPU usage calculation",
			stat1:    "cpu  1000 100 500 5000 200 0 100 0 0 0",
			stat2:    "cpu  1100 110 550 5100 210 0 110 0 0 0",
			expected: 61.54, // (100+10+50) / (100+10+50+100) * 100 = 160/260 * 100 ≈ 61.54%
		},
		{
			name:     "zero delta returns zero usage",
			stat1:    "cpu  1000 100 500 5000 200 0 100 0 0 0",
			stat2:    "cpu  1000 100 500 5000 200 0 100 0 0 0",
			expected: 0.0,
		},
		{
			name:     "insufficient fields returns -1",
			stat1:    "cpu  1000 100",
			stat2:    "cpu  1100 110",
			expected: -1.0,
		},
		{
			name:     "100% CPU usage",
			stat1:    "cpu  1000 100 500 5000 200 0 100 0 0 0",
			stat2:    "cpu  2000 200 1000 5000 400 0 200 0 0 0",
			expected: 100.0,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result := calculateCPUUsage(tc.stat1, tc.stat2)
			if tc.expected == -1.0 {
				suite.Equal(-1.0, result)
			} else {
				suite.InDelta(tc.expected, result, 0.1)
			}
		})
	}
}

func (suite *SysinfoTestSuite) TestFormatSystemInfo() {
	info := &SystemInfo{
		Hostname: "test-host",
		OS:       "Ubuntu 22.04",
		Kernel:   "5.15.0-generic",
		Uptime:   "up 1 day, 2 hours",
		CPUInfo: CPUInfo{
			Model:     "Intel Core i7-8700K",
			Cores:     6,
			Threads:   12,
			LoadAvg1:  "1.23",
			LoadAvg5:  "1.45",
			LoadAvg15: "1.67",
			Usage:     "45.2%",
		},
		MemoryInfo: MemoryInfo{
			TotalMB:      16384,
			UsedMB:       8192,
			FreeMB:       4096,
			AvailableMB:  8192,
			CachedMB:     4096,
			SwapTotalMB:  2048,
			SwapUsedMB:   512,
			SwapFreeMB:   1536,
			UsagePercent: "50.0%",
		},
	}

	result := suite.tool.formatSystemInfo(info, "test-target")
	
	suite.Contains(result, "test-target")
	suite.Contains(result, "test-host")
	suite.Contains(result, "Ubuntu 22.04")
	suite.Contains(result, "5.15.0-generic")
	suite.Contains(result, "Intel Core i7-8700K")
	suite.Contains(result, "6") // cores
	suite.Contains(result, "12") // threads
	suite.Contains(result, "16384") // total memory
	suite.Contains(result, "50.0%") // memory usage
}

func (suite *SysinfoTestSuite) TestNewCreatesValidTool() {
	logger := zerolog.Nop()
	tool := New(logger)
	
	suite.NotNil(tool)
	sysTool, ok := tool.(*Tool)
	suite.True(ok)
	suite.NotNil(sysTool.validator)
	suite.NotNil(sysTool.logger)
}

func TestSysinfoTestSuite(t *testing.T) {
	suite.Run(t, new(SysinfoTestSuite))
}