package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/sniffle6/claude-docket/internal/store"
)

func logSessionHandler(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		summary, ok := argString(args, "summary")
		if !ok || summary == "" {
			return mcp.NewToolResultError("missing required parameter: summary"), nil
		}
		featureID, ok := argString(args, "feature_id")
		if !ok || featureID == "" {
			return mcp.NewToolResultError("missing required parameter: feature_id"), nil
		}

		if _, err := s.GetFeature(featureID); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("feature not found: %s", featureID)), nil
		}

		var filesTouched []string
		if v, ok := argString(args, "files_touched"); ok && v != "" {
			for _, f := range strings.Split(v, ",") {
				filesTouched = append(filesTouched, strings.TrimSpace(f))
			}
		}

		var commits []string
		if v, ok := argString(args, "commits"); ok && v != "" {
			for _, c := range strings.Split(v, ",") {
				commits = append(commits, strings.TrimSpace(c))
			}
		}

		sess, err := s.LogSession(store.SessionInput{
			FeatureID:    featureID,
			Summary:      summary,
			FilesTouched: filesTouched,
			Commits:      commits,
			AutoLinked:   featureID != "",
			LinkReason:   "provided by Claude at session end",
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		s.MarkSessionLogged()
		return mcp.NewToolResultText(fmt.Sprintf("Session #%d logged.", sess.ID)), nil
	}
}

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
