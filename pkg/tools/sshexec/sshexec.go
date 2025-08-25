package sshexec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/connectors/ssh"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/server"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/tools"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/types"
)

type Input struct {
	Host            string   `json:"host" validate:"required,hostname|ip"`  // SSH host (required)
	Port            int      `json:"port,omitempty" validate:"min=0,max=65535"` // SSH port (default: 22)
	User            string   `json:"user,omitempty" validate:"omitempty,alphanum|contains=-|contains=_,max=32"` // SSH user (default: current user)
	BinaryPath      string   `json:"binary_path,omitempty" validate:"omitempty,filepath"` // Local binary path to transfer (required for exec, optional for kill)
	RemotePath      string   `json:"remote_path,omitempty" validate:"omitempty,max=4096"` // Remote destination path (default: /tmp/<filename>)
	Args            []string `json:"args,omitempty"`              // Arguments to pass to the binary
	KeepBinary      bool     `json:"keep_binary,omitempty"`       // Keep binary after execution (default: false, meaning cleanup)
	RunInBackground bool     `json:"run_in_background,omitempty"` // Run process in background (default: false)
	KillPID         int      `json:"kill_pid,omitempty" validate:"min=0,max=2147483647"` // PID to kill on remote host (mutually exclusive with exec)
	KillByName      string   `json:"kill_by_name,omitempty" validate:"omitempty,max=255"` // Kill processes by name pattern (mutually exclusive with exec and kill_pid)
	KillSignal      string   `json:"kill_signal,omitempty" validate:"omitempty,alpha,max=16"` // Signal to send when killing (default: TERM)
	MaxLines        int      `json:"max_lines,omitempty" validate:"min=0,max=100000"` // Maximum lines to return (default: 1000)
	Offset          int      `json:"offset,omitempty" validate:"min=0"` // Line offset for pagination
}

type Output struct {
	Host       string   `json:"host"`
	Port       int      `json:"port"`
	User       string   `json:"user"`
	BinaryPath string   `json:"binary_path"`
	RemotePath string   `json:"remote_path"`
	Args       []string `json:"args"`
	Output     string   `json:"output"`
	ExitCode   int      `json:"exit_code"`
	TotalLines int      `json:"total_lines"`
	Offset     int      `json:"offset"`
	MaxLines   int      `json:"max_lines"`
	Truncated  bool     `json:"truncated"`
}

type Tool struct {
	logger    zerolog.Logger
	validator *validator.Validate
}

func (s *Tool) Register(srv *server.Server) {
	sshExecTool := &mcp.Tool{
		Name:        "sshexec",
		Description: "Transfer and execute a binary on a remote host via SSH, or kill remote processes",
	}

	mcp.AddTool(&srv.Server, sshExecTool, s.SSHExecHandler)
	s.logger.Debug().Msg("sshexec tool registered")
}

func (s *Tool) SSHExecHandler(ctx context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[Input]) (*mcp.CallToolResultFor[Output], error) {
	input := params.Arguments

	// Validate input using validator
	if err := s.validator.Struct(input); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// Additional validation for required fields based on mode
	if input.Host == "" {
		return nil, errors.New("host is required")
	}

	// Determine operation mode
	isKillMode := input.KillPID > 0 || input.KillByName != ""
	isExecMode := input.BinaryPath != ""

	// Validate operation mode
	if isKillMode && isExecMode {
		return nil, errors.New("cannot specify both kill parameters and binary_path - choose either kill or exec mode")
	}
	if !isKillMode && !isExecMode {
		return nil, errors.New("must specify either kill parameters (kill_pid or kill_by_name) or binary_path for execution")
	}
	if input.KillPID > 0 && input.KillByName != "" {
		return nil, errors.New("cannot specify both kill_pid and kill_by_name - choose one")
	}

	// Set defaults
	port := 22
	if input.Port != 0 {
		port = input.Port
	}

	user := input.User

	// Create SSH connector
	conn := ssh.New(input.Host, port, user)

	if isKillMode {
		return s.handleKillMode(ctx, input, conn)
	}

	maxLines := types.MaxDefaultLines
	if input.MaxLines > 0 {
		if input.MaxLines > types.MaxAllowedLines {
			return nil, errors.New("max_lines cannot exceed 100000")
		}
		maxLines = input.MaxLines
	}

	offset := 0
	if input.Offset >= 0 {
		offset = input.Offset
	} else if input.Offset < 0 {
		return nil, errors.New("offset cannot be negative")
	}
	return s.handleExecMode(ctx, input, conn, maxLines, offset)
}

func (s *Tool) handleKillMode(ctx context.Context, input Input, conn *ssh.Connector) (*mcp.CallToolResultFor[Output], error) {
	signal := "TERM"
	if input.KillSignal != "" {
		signal = input.KillSignal
	}

	var remoteCommand string
	var operation string

	if input.KillPID > 0 {
		operation = fmt.Sprintf("kill PID %d", input.KillPID)
		remoteCommand = fmt.Sprintf("kill -%s %d && echo 'Process %d killed with signal %s' || echo 'Failed to kill process %d (may not exist or insufficient permissions)'", signal, input.KillPID, input.KillPID, signal, input.KillPID)
	} else {
		operation = fmt.Sprintf("kill processes matching '%s'", input.KillByName)
		// Use pkill which is safer and more precise than killall
		// -x flag matches exact process name
		// Properly escape the process name to prevent shell injection
		escapedName := ssh.EscapeArg(input.KillByName)
		remoteCommand = fmt.Sprintf("pkill -%s -x %s && echo 'Processes matching \"%s\" killed with signal %s' || echo 'No processes found matching \"%s\" or kill failed'", signal, escapedName, input.KillByName, signal, input.KillByName)
	}

	s.logger.Debug().
		Str("host", input.Host).
		Str("operation", operation).
		Str("signal", signal).
		Msg("killing remote process(es)")

	output, exitCode, err := conn.ExecuteCommandWithExitCode(ctx, remoteCommand)
	if err != nil {
		return nil, fmt.Errorf("failed to execute kill command: %v", err)
	}

	resultText := fmt.Sprintf("SSH Kill output for %s (operation: %s):\n", conn.GetTarget(), operation)
	resultText += fmt.Sprintf("Exit Code: %d\n", exitCode)
	resultText += "\n" + strings.TrimSpace(output)

	return &mcp.CallToolResultFor[Output]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: resultText,
			},
		},
	}, nil
}

func (s *Tool) handleExecMode(ctx context.Context, input Input, conn *ssh.Connector, maxLines, offset int) (*mcp.CallToolResultFor[Output], error) {
	// Check if binary exists
	if _, err := os.Stat(input.BinaryPath); err != nil {
		return nil, fmt.Errorf("binary not found: %v", err)
	}

	remotePath := input.RemotePath
	if remotePath == "" {
		remotePath = filepath.Join("/tmp", filepath.Base(input.BinaryPath))
	}

	cleanup := !input.KeepBinary // cleanup by default unless KeepBinary is true

	s.logger.Debug().
		Str("host", input.Host).
		Str("user", conn.GetTarget()).
		Str("binary", input.BinaryPath).
		Str("remote", remotePath).
		Bool("cleanup", cleanup).
		Msg("transferring and executing binary")

	// Step 1: Transfer binary using scp
	if err := conn.CopyFile(ctx, input.BinaryPath, remotePath); err != nil {
		return nil, err
	}

	// Step 2: Make binary executable
	if err := conn.MakeExecutable(ctx, remotePath); err != nil {
		// Clean up the transferred binary on error
		_ = conn.RemoveFile(ctx, remotePath)
		return nil, fmt.Errorf("failed to make binary executable: %v", err)
	}

	// Step 3: Execute binary with arguments
	remoteCommand := remotePath
	if len(input.Args) > 0 {
		escapedArgs := ssh.EscapeArgs(input.Args)
		remoteCommand = fmt.Sprintf("%s %s", remotePath, strings.Join(escapedArgs, " "))
	}

	// Add background execution if requested
	if input.RunInBackground {
		remoteCommand = fmt.Sprintf("nohup %s > /dev/null 2>&1 & echo $!", remoteCommand)
		if cleanup {
			// For background processes, we can't easily clean up after exit, so warn user
			s.logger.Warn().Msg("cleanup disabled for background processes - binary will remain on remote host")
		}
	} else if cleanup {
		// Add cleanup command if requested (only for foreground processes)
		remoteCommand = fmt.Sprintf("%s; EXIT_CODE=$?; rm -f %s; exit $EXIT_CODE", remoteCommand, remotePath)
	}

	output, exitCode, err := conn.ExecuteCommandWithExitCode(ctx, remoteCommand)
	if err != nil {
		return nil, fmt.Errorf("failed to execute binary: %v", err)
	}

	resultText := fmt.Sprintf("SSH Exec output for %s (binary: %s):\n", conn.GetTarget(), filepath.Base(input.BinaryPath))

	if input.RunInBackground {
		resultText += fmt.Sprintf("Process started in background. PID: %s\n", strings.TrimSpace(output))
		resultText += "Note: Binary will remain on remote host for background processes."
	} else {
		// Apply pagination
		lines := strings.Split(output, "\n")
		totalLines := len(lines)

		if offset >= totalLines {
			lines = []string{}
		} else {
			end := offset + maxLines
			if end > totalLines {
				end = totalLines
			}
			lines = lines[offset:end]
		}

		truncated := totalLines > (offset + maxLines)
		paginatedOutput := strings.Join(lines, "\n")

		resultText += fmt.Sprintf("Exit Code: %d\n", exitCode)
		if truncated {
			resultText += fmt.Sprintf("[Showing lines %d-%d of %d total lines. Use offset parameter to view more.]\n", offset+1, offset+len(lines), totalLines)
		}
		resultText += "\n" + strings.TrimSpace(paginatedOutput)
	}

	return &mcp.CallToolResultFor[Output]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: resultText,
			},
		},
	}, nil
}

func New(logger zerolog.Logger) tools.Tool {
	validate := validator.New()

	return &Tool{
		logger:    logger.With().Str("tool", "sshexec").Logger(),
		validator: validate,
	}
}
