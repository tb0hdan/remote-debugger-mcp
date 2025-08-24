package server

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Server struct {
	mcp.Server
}

func NewServer(impl *mcp.Implementation) *Server {
	return &Server{
		Server: *mcp.NewServer(impl, nil),
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	return nil
}
