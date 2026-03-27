package mcp

import (
	"github.com/mark3labs/mcp-go/server"

	"github.com/sniffyanimal/feat/internal/store"
)

func NewServer(s *store.Store) *server.MCPServer {
	srv := server.NewMCPServer("feat", "0.1.0",
		server.WithToolCapabilities(true),
	)

	registerTools(srv, s)
	return srv
}
