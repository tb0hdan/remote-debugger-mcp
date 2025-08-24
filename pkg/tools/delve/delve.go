package delve

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/server"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/tools"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/types"
)

type Input struct {
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Command  string `json:"command,omitempty"`
	MaxLines int    `json:"max_lines,omitempty"` // Maximum lines to return (default: 1000)
	Offset   int    `json:"offset,omitempty"`    // Line offset for pagination
}

type Output struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Command    string `json:"command"`
	Output     string `json:"output"`
	TotalLines int    `json:"total_lines"`
	Offset     int    `json:"offset"`
	MaxLines   int    `json:"max_lines"`
	Truncated  bool   `json:"truncated"`
}

type Tool struct {
	logger zerolog.Logger
}

func (d *Tool) DelveHandler(_ context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[Input]) (*mcp.CallToolResultFor[Output], error) {
	input := params.Arguments

	host := "localhost"
	if input.Host != "" {
		host = input.Host
	}

	port := 2345
	if input.Port != 0 {
		port = input.Port
	}

	command := "help"
	if input.Command != "" {
		command = input.Command
	}

	addr := fmt.Sprintf("%s:%d", host, port)

	d.logger.Info().Msgf("Connecting to Delve debugger at %s with command: %s", addr, command)

	cmd := exec.Command("dlv", "connect", addr)

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

func (d *Tool) Register(srv *server.Server) {
	delveTool := &mcp.Tool{
		Name:        "delve",
		Description: "Connects to a remote Delve debugger using dlv command",
	}

	mcp.AddTool(&srv.Server, delveTool, d.DelveHandler)
	d.logger.Debug().Msg("delve tool registered")
}

func New(logger zerolog.Logger) tools.Tool {
	return &Tool{
		logger: logger.With().Str("tool", "delve").Logger(),
	}
}
