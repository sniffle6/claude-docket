package mcp

import (
	"github.com/mark3labs/mcp-go/server"

	"github.com/sniffle6/claude-docket/internal/store"
)

func NewServer(s *store.Store, projectDir string, onCheckpoint func()) *server.MCPServer {
	srv := server.NewMCPServer("docket", "0.1.0",
		server.WithToolCapabilities(true),
	)

	registerTools(srv, s, projectDir, onCheckpoint)
	return srv
}
