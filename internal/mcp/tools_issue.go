package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/sniffle6/claude-docket/internal/store"
)

func addIssueHandler(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		featureID, ok := argString(args, "feature_id")
		if !ok || featureID == "" {
			return mcp.NewToolResultError("missing required parameter: feature_id"), nil
		}
		description, ok := argString(args, "description")
		if !ok || description == "" {
			return mcp.NewToolResultError("missing required parameter: description"), nil
		}

		var taskItemID *int64
		if v, ok := argString(args, "task_item_id"); ok && v != "" {
			id := parseInt64(v)
			taskItemID = &id
		}

		issue, err := s.AddIssue(featureID, description, taskItemID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		msg := fmt.Sprintf("Issue #%d logged on %s: %s", issue.ID, featureID, description)
		if taskItemID != nil {
			msg += fmt.Sprintf(" (linked to task item #%d)", *taskItemID)
		}
		return mcp.NewToolResultText(msg), nil
	}
}

func resolveIssueHandler(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		idStr, ok := argString(args, "id")
		if !ok || idStr == "" {
			return mcp.NewToolResultError("missing required parameter: id"), nil
		}
		id := parseInt64(idStr)
		commitHash, _ := argString(args, "commit_hash")

		if err := s.ResolveIssue(id, commitHash); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		msg := fmt.Sprintf("Issue #%d resolved.", id)
		if commitHash != "" {
			msg += fmt.Sprintf(" Fix: %s", commitHash)
		}
		return mcp.NewToolResultText(msg), nil
	}
}

func listIssuesHandler(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		featureID, _ := argString(req.GetArguments(), "feature_id")

		var issues []store.Issue
		var err error
		if featureID != "" {
			issues, err = s.GetIssuesForFeature(featureID)
		} else {
			issues, err = s.GetAllOpenIssues()
		}
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if len(issues) == 0 {
			return mcp.NewToolResultText("No open issues."), nil
		}

		var lines []string
		for _, iss := range issues {
			line := fmt.Sprintf("- #%d [%s] %s", iss.ID, iss.FeatureID, iss.Description)
			if iss.Status == "resolved" {
				line += " (resolved"
				if iss.ResolvedCommit != "" {
					line += ": " + iss.ResolvedCommit
				}
				line += ")"
			}
			lines = append(lines, line)
		}
		return mcp.NewToolResultText(strings.Join(lines, "\n")), nil
	}
}

func addDecisionHandler(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		featureID, ok := argString(args, "feature_id")
		if !ok || featureID == "" {
			return mcp.NewToolResultError("missing required parameter: feature_id"), nil
		}
		approach, ok := argString(args, "approach")
		if !ok || approach == "" {
			return mcp.NewToolResultError("missing required parameter: approach"), nil
		}
		outcome, ok := argString(args, "outcome")
		if !ok || outcome == "" {
			return mcp.NewToolResultError("missing required parameter: outcome"), nil
		}
		reason, ok := argString(args, "reason")
		if !ok || reason == "" {
			return mcp.NewToolResultError("missing required parameter: reason"), nil
		}

		if outcome != "accepted" && outcome != "rejected" {
			return mcp.NewToolResultError("outcome must be 'accepted' or 'rejected'"), nil
		}

		d, err := s.AddDecision(featureID, approach, outcome, reason)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Decision #%d logged: %s %s — %s", d.ID, outcome, approach, reason)), nil
	}
}
