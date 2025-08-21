package profiler

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
)

type PprofInput struct {
	Host    string `json:"host,omitempty"`
	Port    int    `json:"port,omitempty"`
	Profile string `json:"profile,omitempty"`
	Seconds int    `json:"seconds,omitempty"`
}

type PprofOutput struct {
	URL         string `json:"url"`
	Status      int    `json:"status"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
	Content     string `json:"content"`
}

type PprofTool struct {
	logger zerolog.Logger
}

func (p *PprofTool) PprofHandler(_ context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[PprofInput]) (*mcp.CallToolResultFor[PprofOutput], error) {
	input := params.Arguments

	host := "localhost"
	if input.Host != "" {
		host = input.Host
	}

	port := 6060
	if input.Port != 0 {
		port = input.Port
	}

	profile := "heap"
	if input.Profile != "" {
		profile = input.Profile
	}

	seconds := 30
	if input.Seconds != 0 {
		seconds = input.Seconds
	}

	baseURL := fmt.Sprintf("http://%s:%d/debug/pprof/", host, port)

	var profileURL string
	switch profile {
	case "profile":
		profileURL = fmt.Sprintf("%sprofile?seconds=%d", baseURL, seconds)
	default:
		profileURL = baseURL + profile
	}

	p.logger.Info().Msgf("Sending request to %s", profileURL)
	args := []string{"tool", "pprof", "-top", profileURL}
	cmd := exec.Command("go", args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to execute go tool pprof: %w\nOutput: %s", err, string(output))
	}

	result := &mcp.CallToolResultFor[PprofOutput]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("go tool pprof output for %s:\n\n%s", profileURL, strings.TrimSpace(string(output))),
			},
		},
	}

	return result, nil
}

func NewPprofTool(logger zerolog.Logger) *PprofTool {
	return &PprofTool{
		logger: logger,
	}
}
