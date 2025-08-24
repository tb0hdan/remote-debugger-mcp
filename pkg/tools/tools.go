package tools

import (
	"github.com/tb0hdan/remote-debugger-mcp/pkg/server"
)

type Tool interface {
	Register(srv *server.Server)
}
