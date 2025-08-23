package pprof

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/tools"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/types"
)

type Input struct {
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Profile  string `json:"profile,omitempty"`
	Seconds  int    `json:"seconds,omitempty"`
	MaxLines int    `json:"max_lines,omitempty"` // Maximum lines to return (default: 100 for top view)
	Offset   int    `json:"offset,omitempty"`    // Line offset for pagination
}

type Output struct {
	URL         string `json:"url"`
	Status      int    `json:"status"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
	Content     string `json:"content"`
	TotalLines  int    `json:"total_lines"`
	Offset      int    `json:"offset"`
	MaxLines    int    `json:"max_lines"`
	Truncated   bool   `json:"truncated"`
}

type Tool struct {
	logger zerolog.Logger
}

// fetchAvailableProfiles fetches the pprof index page and extracts available profile links.
func (p *Tool) fetchAvailableProfiles(baseURL string) ([]string, error) {
	resp, err := http.Get(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pprof index page: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch pprof index page: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read pprof index page: %w", err)
	}

	// Extract profile links from HTML
	// Looking for patterns like href="/debug/pprof/profile" or href="profile"
	re := regexp.MustCompile(`href="(?:/debug/pprof/)?([^"]+)"`)
	matches := re.FindAllStringSubmatch(string(body), -1)

	profiles := []string{}
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) > 1 {
			profile := match[1]
			// Skip non-profile links
			if strings.Contains(profile, "/") || strings.Contains(profile, "http") {
				continue
			}
			// Skip duplicate entries
			if seen[profile] {
				continue
			}
			seen[profile] = true
			profiles = append(profiles, profile)
		}
	}

	return profiles, nil
}

func (p *Tool) PprofHandler(_ context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[Input]) (*mcp.CallToolResultFor[Output], error) {
	input := params.Arguments

	host := "localhost"
	if input.Host != "" {
		host = input.Host
	}

	port := 6060
	if input.Port != 0 {
		port = input.Port
	}

	profile := ""
	if input.Profile != "" {
		profile = input.Profile
	}

	seconds := 30
	if input.Seconds != 0 {
		seconds = input.Seconds
	}

	baseURL := fmt.Sprintf("http://%s:%d/debug/pprof/", host, port)

	// If no profile specified or "list" is requested, return available profiles
	if profile == "" || profile == "list" {
		profiles, err := p.fetchAvailableProfiles(baseURL)
		if err != nil {
			return nil, err
		}

		resultText := fmt.Sprintf("Available pprof profiles at %s:\n\n", baseURL)
		for _, prof := range profiles {
			resultText += fmt.Sprintf("  - %s\n", prof)
		}
		resultText += "\nTo retrieve a specific profile, use the 'profile' parameter.\n"
		resultText += "Example: profile='heap' or profile='goroutine'\n"
		resultText += "For CPU profiling, use profile='profile' with optional seconds parameter."

		result := &mcp.CallToolResultFor[Output]{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: resultText,
				},
			},
		}
		return result, nil
	}

	var profileURL string
	switch profile {
	case "profile":
		profileURL = fmt.Sprintf("%sprofile?seconds=%d", baseURL, seconds)
	default:
		profileURL = baseURL + profile
	}

	// Determine max lines for pagination (default: 100 for top view)
	maxLines := types.MaxDefaultLines
	if input.MaxLines > 0 {
		maxLines = input.MaxLines
	}

	// Use -text flag to get text output from pprof
	p.logger.Info().Msgf("Sending request to %s", profileURL)
	args := []string{"tool", "pprof", "-text", profileURL}
	cmd := exec.Command("go", args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to execute go tool pprof: %w\nOutput: %s", err, string(output))
	}

	// Apply pagination
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	totalLines := len(lines)

	offset := 0
	if input.Offset > 0 {
		offset = input.Offset
	}

	// Apply offset if needed
	truncated := false
	if offset > 0 && offset < totalLines {
		end := totalLines
		if offset+maxLines < totalLines {
			end = offset + maxLines
			truncated = true
		}
		lines = lines[offset:end]
	} else if totalLines > maxLines {
		lines = lines[:maxLines]
		truncated = true
	}

	paginatedOutput := strings.Join(lines, "\n")

	resultText := fmt.Sprintf("go tool pprof output for %s:\n", profileURL)
	if truncated || offset > 0 {
		resultText += fmt.Sprintf("[Showing lines %d-%d of approximately %d lines. Use offset parameter to view more.]\n", offset+1, offset+len(lines), totalLines)
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

func (p *Tool) Register(server *mcp.Server) {
	tool := &mcp.Tool{
		Name:        "pprof",
		Description: "Connects to a remote pprof server and retrieves profiling data",
	}

	mcp.AddTool(server, tool, p.PprofHandler)
	p.logger.Debug().Msg("pprof tool registered")
}

func New(logger zerolog.Logger) tools.Tool {
	return &Tool{
		logger: logger.With().Str("tool", "pprof").Logger(),
	}
}
