package delve

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/server"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/tools"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/types"
)

const (
	sessionStartupDelay = 500 * time.Millisecond
	commandTimeout      = 5 * time.Second
	disconnectDelay     = 100 * time.Millisecond
	cleanupInterval     = 5 * time.Minute
	sessionMaxIdleTime  = 30 * time.Minute
)

type Input struct {
	Host      string `json:"host,omitempty" validate:"omitempty,hostname|ip"`
	Port      int    `json:"port,omitempty" validate:"min=0,max=65535"`
	Command   string `json:"command,omitempty" validate:"max=4096"`
	SessionID string `json:"session_id,omitempty" validate:"omitempty,alphanum,max=64"` // Session ID for persistent connections
	Action    string `json:"action,omitempty" validate:"omitempty,oneof=connect disconnect command"` // Action: connect, disconnect, or command (default: command)
	MaxLines  int    `json:"max_lines,omitempty" validate:"min=0,max=100000"` // Maximum lines to return (default: 1000)
	Offset    int    `json:"offset,omitempty" validate:"min=0"`                 // Line offset for pagination
}

type Output struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Command    string `json:"command,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
	Action     string `json:"action,omitempty"`
	Output     string `json:"output"`
	TotalLines int    `json:"total_lines"`
	Offset     int    `json:"offset"`
	MaxLines   int    `json:"max_lines"`
	Truncated  bool   `json:"truncated"`
	Status     string `json:"status"` // Session status: connected, disconnected, command_executed
}

// DelveSession represents a persistent Delve debugger connection.
type DelveSession struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   io.ReadCloser
	stderr   io.ReadCloser
	scanner  *bufio.Scanner
	host     string
	port     int
	mu       sync.Mutex
	lastUsed time.Time
}

type Tool struct {
	logger    zerolog.Logger
	validator *validator.Validate
	sessions  map[string]*DelveSession
	sessionMu sync.RWMutex
}

func (d *Tool) DelveHandler(ctx context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[Input]) (*mcp.CallToolResultFor[Output], error) {
	input := params.Arguments

	// Validate input using validator
	if err := d.validator.Struct(input); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	host := "localhost"
	if input.Host != "" {
		host = input.Host
	}

	port := 2345
	if input.Port != 0 {
		port = input.Port
	}

	action := "command"
	if input.Action != "" {
		action = input.Action
	}

	// Handle session-based operations
	if input.SessionID != "" {
		return d.handleSessionOperation(ctx, input, host, port, action)
	}

	// Non-session mode (backward compatibility)
	return d.handleSingleCommand(ctx, input, host, port)
}

func (d *Tool) Register(srv *server.Server) {
	delveTool := &mcp.Tool{
		Name:        "delve",
		Description: "Connects to a remote Delve debugger with session support for interactive debugging",
	}

	mcp.AddTool(&srv.Server, delveTool, d.DelveHandler)
	d.logger.Debug().Msg("delve tool registered")

	// Start cleanup goroutine for stale sessions
	go d.cleanupStaleSessions()
}

// cleanupStaleSessions removes sessions that haven't been used for 30 minutes.
func (d *Tool) cleanupStaleSessions() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		d.sessionMu.Lock()
		now := time.Now()
		for sessionID, session := range d.sessions {
			if now.Sub(session.lastUsed) > sessionMaxIdleTime {
				d.logger.Info().Msgf("Cleaning up stale session %s", sessionID)
				// Send exit command
				if session.stdin != nil {
					_, _ = fmt.Fprintf(session.stdin, "exit\n")
					time.Sleep(disconnectDelay)
					_ = session.stdin.Close()
				}
				// Cleanup
				if session.cmd != nil && session.cmd.Process != nil {
					_ = session.cmd.Process.Kill()
				}
				delete(d.sessions, sessionID)
			}
		}
		d.sessionMu.Unlock()
	}
}

// connectSession creates a new Delve session.
func (d *Tool) connectSession(ctx context.Context, sessionID, host string, port int) (*DelveSession, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	d.logger.Info().Msgf("Creating new Delve session %s at %s", sessionID, addr)

	cmd := exec.CommandContext(ctx, "dlv", "connect", addr)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start dlv command: %w", err)
	}

	session := &DelveSession{
		cmd:      cmd,
		stdin:    stdin,
		stdout:   stdout,
		stderr:   stderr,
		scanner:  bufio.NewScanner(stdout),
		host:     host,
		port:     port,
		lastUsed: time.Now(),
	}

	// Read initial prompt
	time.Sleep(sessionStartupDelay)
	
	return session, nil
}

// executeCommand sends a command to an existing session and reads the response.
func (d *Tool) executeCommand(session *DelveSession, command string) (string, error) {
	session.mu.Lock()
	defer session.mu.Unlock()

	session.lastUsed = time.Now()

	// Send command
	if _, err := fmt.Fprintf(session.stdin, "%s\n", command); err != nil {
		return "", fmt.Errorf("failed to send command: %w", err)
	}

	// Read response with timeout
	var output strings.Builder
	done := make(chan bool)
	var readErr error

	go func() {
		scanner := bufio.NewScanner(session.stdout)
		for scanner.Scan() {
			line := scanner.Text()
			output.WriteString(line)
			output.WriteString("\n")
			
			// Check for prompt indicating command completion
			if strings.Contains(line, "(dlv)") || strings.Contains(line, ">") {
				done <- true
				return
			}
		}
		if err := scanner.Err(); err != nil {
			readErr = err
		}
		done <- false
	}()

	// Wait for response with timeout
	select {
	case <-done:
		if readErr != nil {
			return output.String(), readErr
		}
	case <-time.After(commandTimeout):
		return output.String(), errors.New("command timed out")
	}

	return output.String(), nil
}

// disconnectSession closes a Delve session.
func (d *Tool) disconnectSession(sessionID string) error {
	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()

	session, exists := d.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	// Send exit command
	if session.stdin != nil {
		_, _ = fmt.Fprintf(session.stdin, "exit\n")
		time.Sleep(disconnectDelay)
		_ = session.stdin.Close()
	}

	// Cleanup
	if session.cmd != nil && session.cmd.Process != nil {
		_ = session.cmd.Process.Kill()
	}

	delete(d.sessions, sessionID)
	d.logger.Info().Msgf("Disconnected Delve session %s", sessionID)
	return nil
}

// handleSessionOperation handles session-based operations.
func (d *Tool) handleSessionOperation(ctx context.Context, input Input, host string, port int, action string) (*mcp.CallToolResultFor[Output], error) {
	switch action {
	case "connect":
		return d.handleConnect(ctx, input, host, port)
	case "disconnect":
		return d.handleDisconnect(input)
	case "command":
		return d.handleCommand(input)
	default:
		return nil, fmt.Errorf("unsupported action: %s. Use 'connect', 'disconnect', or 'command'", action)
	}
}

// handleConnect creates a new Delve session.
func (d *Tool) handleConnect(ctx context.Context, input Input, host string, port int) (*mcp.CallToolResultFor[Output], error) {
	d.sessionMu.Lock()
	if _, exists := d.sessions[input.SessionID]; exists {
		d.sessionMu.Unlock()
		return nil, fmt.Errorf("session %s already exists", input.SessionID)
	}

	session, err := d.connectSession(ctx, input.SessionID, host, port)
	if err != nil {
		d.sessionMu.Unlock()
		return nil, err
	}

	d.sessions[input.SessionID] = session
	d.sessionMu.Unlock()

	resultText := fmt.Sprintf("Connected to Delve debugger at %s:%d\nSession ID: %s\nSession established. Use 'command' action to send debugging commands.", host, port, input.SessionID)

	result := &mcp.CallToolResultFor[Output]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: resultText,
			},
		},
	}

	return result, nil
}

// handleDisconnect disconnects a Delve session.
func (d *Tool) handleDisconnect(input Input) (*mcp.CallToolResultFor[Output], error) {
	if err := d.disconnectSession(input.SessionID); err != nil {
		return nil, err
	}

	resultText := "Disconnected Delve session: " + input.SessionID

	result := &mcp.CallToolResultFor[Output]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: resultText,
			},
		},
	}

	return result, nil
}

// handleCommand executes a command in an existing session.
func (d *Tool) handleCommand(input Input) (*mcp.CallToolResultFor[Output], error) {
	d.sessionMu.RLock()
	session, exists := d.sessions[input.SessionID]
	d.sessionMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session %s not found. Use 'connect' action first", input.SessionID)
	}

	command := "help"
	if input.Command != "" {
		command = input.Command
	}

	output, err := d.executeCommand(session, command)
	if err != nil {
		return nil, fmt.Errorf("failed to execute command: %w", err)
	}

	// Apply pagination
	maxLines := types.MaxDefaultLines
	if input.MaxLines > 0 {
		maxLines = input.MaxLines
	}

	offset := 0
	if input.Offset > 0 {
		offset = input.Offset
	}

	lines := strings.Split(output, "\n")
	totalLines := len(lines)

	// Apply offset and limit
	truncated := false
	if offset < totalLines {
		end := offset + maxLines
		if end > totalLines {
			end = totalLines
		} else {
			truncated = true
		}
		lines = lines[offset:end]
	} else {
		lines = []string{}
	}

	paginatedOutput := strings.Join(lines, "\n")

	resultText := fmt.Sprintf("Session %s - Command: %s\n", input.SessionID, command)
	if truncated {
		resultText += fmt.Sprintf("[Showing lines %d-%d of %d total lines. Use offset parameter to view more.]\n", offset+1, offset+len(lines), totalLines)
	}
	resultText += "\n" + strings.TrimSpace(paginatedOutput)

	result := &mcp.CallToolResultFor[Output]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: resultText,
			},
		},
	}

	return result, nil
}

// handleSingleCommand handles a single command without session.
func (d *Tool) handleSingleCommand(ctx context.Context, input Input, host string, port int) (*mcp.CallToolResultFor[Output], error) {
	command := "help"
	if input.Command != "" {
		command = input.Command
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	d.logger.Info().Msgf("Connecting to Delve debugger at %s with command: %s", addr, command)

	cmd := exec.CommandContext(ctx, "dlv", "connect", addr)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start dlv command: %w", err)
	}

	commands := command + "\nexit\n"
	if _, err := io.WriteString(stdin, commands); err != nil {
		return nil, fmt.Errorf("failed to write commands to stdin: %w", err)
	}

	err = stdin.Close()
	if err != nil {
		return nil, err
	}

	var outputBuf, errorBuf bytes.Buffer
	go func() {
		_, err := io.Copy(&outputBuf, stdout)
		if err != nil {
			d.logger.Error().Err(err).Msg("Failed to read from stdout")
		}
	}()
	go func() {
		_, err := io.Copy(&errorBuf, stderr)
		if err != nil {
			d.logger.Error().Err(err).Msg("Failed to read from stderr")
		}
	}()

	if err := cmd.Wait(); err != nil {
		if errorBuf.Len() > 0 {
			return nil, fmt.Errorf("dlv command failed: %w\nError: %s", err, errorBuf.String())
		}
	}

	output := outputBuf.String()
	if errorBuf.Len() > 0 {
		output += "\nErrors:\n" + errorBuf.String()
	}

	// Apply pagination
	maxLines := types.MaxDefaultLines
	if input.MaxLines > 0 {
		maxLines = input.MaxLines
	}

	offset := 0
	if input.Offset > 0 {
		offset = input.Offset
	}

	lines := strings.Split(output, "\n")
	totalLines := len(lines)

	// Apply offset and limit
	truncated := false
	if offset < totalLines {
		end := offset + maxLines
		if end > totalLines {
			end = totalLines
		} else {
			truncated = true
		}
		lines = lines[offset:end]
	} else {
		lines = []string{}
	}

	paginatedOutput := strings.Join(lines, "\n")

	resultText := fmt.Sprintf("Delve debugger output for %s (command: %s):\n", addr, command)
	if truncated {
		resultText += fmt.Sprintf("[Showing lines %d-%d of %d total lines. Use offset parameter to view more.]\n", offset+1, offset+len(lines), totalLines)
	}
	resultText += "\n" + strings.TrimSpace(paginatedOutput)

	result := &mcp.CallToolResultFor[Output]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: resultText,
			},
		},
	}

	return result, nil
}

func New(logger zerolog.Logger) tools.Tool {
	validate := validator.New()

	return &Tool{
		logger:    logger.With().Str("tool", "delve").Logger(),
		validator: validate,
		sessions:  make(map[string]*DelveSession),
	}
}