package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/tools/delve"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/tools/profiler"
)

const (
	ServerName  = "remote-debugger-mcp"
	ServiceName = "Remote Debugger MCP Connector"
)

//go:embed VERSION
var Version string

func main() {
	var (
		debug        bool
		port         int
		printVersion bool
	)
	flag.BoolVar(&debug, "debug", false, "debug mode")
	flag.IntVar(&port, "port", 8899, "server port")
	flag.BoolVar(&printVersion, "version", false, "print version and exit")
	flag.Parse()
	// Sanitize version
	version := strings.TrimSpace(Version)
	// Check if the version flag is set
	if printVersion {
		fmt.Printf("%s Version: %s", ServiceName, version)
		os.Exit(0)
	}

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		logger.Debug().Msg("debug mode enabled")
	}

	impl := &mcp.Implementation{
		Name:    ServerName,
		Version: version,
	}

	server := mcp.NewServer(impl, nil)
	// Register tools
	pprofTool := profiler.New(logger)
	pprofTool.Register(server)
	// Add more tools as needed
	delveTool := delve.New(logger)
	delveTool.Register(server)

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, nil)

	http.Handle("/mcp", handler)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"service": ServiceName,
			"version": version,
			"endpoints": map[string]string{
				"mcp": "/mcp",
			},
		})
	})

	logger.Info().Msgf("%s starting on port %d", ServiceName, port)
	logger.Info().Msgf("MCP endpoint available at: http://localhost:%d/mcp", port)

	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); !errors.Is(err, http.ErrServerClosed) {
		logger.Fatal().Msgf("%s failed to start: %v", ServerName, err)
	}
}
