package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	_ "net/http/pprof"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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

func pprofHandler(_ context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[PprofInput]) (*mcp.CallToolResultFor[PprofOutput], error) {
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

        fmt.Println(profileURL)
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

func createServer() *mcp.Server {
	impl := &mcp.Implementation{
		Name:    "go-mcp",
		Version: "1.0.0",
	}

	server := mcp.NewServer(impl, nil)

	pprofTool := &mcp.Tool{
		Name:        "pprof",
		Description: "Executes go tool pprof to analyze profiling data from another application",
	}

	mcp.AddTool(server, pprofTool, pprofHandler)

	return server
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	port := 8899
	if portStr := getEnv("PORT", ""); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	server := createServer()

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, nil)

	http.Handle("/mcp", handler)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"service": "MCP Pprof Connector",
			"version": "1.0.0",
			"endpoints": map[string]string{
				"mcp": "/mcp",
			},
		})
	})

	log.Printf("MCP Pprof Connector starting on port %d", port)
	log.Printf("MCP endpoint available at: http://localhost:%d/mcp", port)

	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
