package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/sniffle6/claude-docket/internal/store"
)

func addNoteHandler(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		featureID, ok := argString(args, "feature_id")
		if !ok || featureID == "" {
			return mcp.NewToolResultError("missing required parameter: feature_id"), nil
		}
		content, ok := argString(args, "content")
		if !ok || content == "" {
			return mcp.NewToolResultError("missing required parameter: content"), nil
		}

		note, err := s.AddNote(featureID, content)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Note #%d added to %s", note.ID, featureID)), nil
	}
}
