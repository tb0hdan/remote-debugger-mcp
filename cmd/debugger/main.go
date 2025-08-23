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
	"github.com/tb0hdan/remote-debugger-mcp/pkg/tools"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/tools/delve"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/tools/pprof"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/tools/sshexec"
	"github.com/tb0hdan/remote-debugger-mcp/pkg/tools/sysinfo"
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
		bindAddr     string
		printVersion bool
	)
	flag.BoolVar(&debug, "debug", false, "debug mode")
	flag.StringVar(&bindAddr, "bind", "localhost:8899", "bind address (host:port)")
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
	toolList := []tools.Tool{
		pprof.New(logger),
		delve.New(logger),
		sshexec.New(logger),
		sysinfo.New(logger),
	}
	// Register all tools
	for _, tool := range toolList {
		tool.Register(server)
	}
	// Create HTTP handler for MCP server
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

	logger.Info().Msgf("%s starting on address %s", ServiceName, bindAddr)
	logger.Info().Msgf("MCP endpoint available at: http://%s/mcp", bindAddr)

	if err := http.ListenAndServe(bindAddr, nil); !errors.Is(err, http.ErrServerClosed) {
		logger.Fatal().Msgf("%s failed to start: %v", ServerName, err)
	}
}
