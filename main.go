package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/profiler"
)

func createServer(logger zerolog.Logger) *mcp.Server {
	impl := &mcp.Implementation{
		Name:    "remote-debugger-mcp",
		Version: "1.0.0",
	}

	server := mcp.NewServer(impl, nil)

	pprofTool := &mcp.Tool{
		Name:        "pprof",
		Description: "Executes go tool pprof to analyze profiling data from another application",
	}

	tool := profiler.NewPprofTool(logger)
	mcp.AddTool(server, pprofTool, tool.PprofHandler)

	return server
}

func main() {
	var (
		debug bool
		port  int
	)
	flag.BoolVar(&debug, "debug", false, "debug mode")
	flag.IntVar(&port, "port", 8899, "server port")
	flag.Parse()

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		logger.Debug().Msg("debug mode enabled")
	}
	server := createServer(logger)

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, nil)

	http.Handle("/mcp", handler)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"service": "Remote Debugger MCP Connector",
			"version": "1.0.0",
			"endpoints": map[string]string{
				"mcp": "/mcp",
			},
		})
	})

	logger.Info().Msgf("Remote Debugger MCP Connector starting on port %d", port)
	logger.Info().Msgf("MCP endpoint available at: http://localhost:%d/mcp", port)

	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); !errors.Is(err, http.ErrServerClosed) {
		logger.Fatal().Msgf("Server failed to start:", err)
	}
}
