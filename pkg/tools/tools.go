package tools

import "github.com/modelcontextprotocol/go-sdk/mcp"

type Tool interface {
	Register(server *mcp.Server)
}
