package mcp

import (
	"github.com/mark3labs/mcp-go/server"

	"github.com/sniffle6/claude-docket/internal/store"
)

func NewServer(s *store.Store) *server.MCPServer {
	srv := server.NewMCPServer("docket", "0.1.0",
		server.WithToolCapabilities(true),
	)

	registerTools(srv, s)
	return srv
}
