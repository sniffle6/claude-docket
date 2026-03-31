package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/sniffle6/claude-docket/internal/store"
)

func compactSessionsHandler(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		id, ok := argString(args, "id")
		if !ok || id == "" {
			return mcp.NewToolResultError("missing required parameter: id"), nil
		}
		summary, ok := argString(args, "summary")
		if !ok || summary == "" {
			return mcp.NewToolResultError("missing required parameter: summary"), nil
		}

		n, err := s.CompactSessions(id, summary)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if n == 0 {
			return mcp.NewToolResultText("Nothing to compact (3 or fewer sessions)."), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Compacted %d sessions into 1 summary. Last 3 sessions preserved.", n)), nil
	}
}
