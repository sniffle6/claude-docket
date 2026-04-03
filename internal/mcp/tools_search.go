package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/sniffle6/claude-docket/internal/store"
)

func searchHandler(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		query, ok := argString(args, "query")
		if !ok || query == "" {
			return mcp.NewToolResultError("missing required parameter: query"), nil
		}

		opts := store.SearchOpts{}

		if scopeStr, ok := argString(args, "scope"); ok && scopeStr != "" {
			for _, sc := range strings.Split(scopeStr, ",") {
				sc = strings.TrimSpace(sc)
				if sc != "" {
					opts.Scope = append(opts.Scope, sc)
				}
			}
		}

		if featureID, ok := argString(args, "feature_id"); ok && featureID != "" {
			opts.FeatureID = featureID
		}

		if v, ok := args["verbose"]; ok {
			if b, ok := v.(bool); ok {
				opts.Verbose = b
			}
		}

		if limitStr, ok := argString(args, "limit"); ok && limitStr != "" {
			opts.Limit = int(parseInt64(limitStr))
		}

		results, err := s.Search(query, opts)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
		}

		if len(results) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No results found for %q", query)), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Found %d results for %q:\n", len(results), query)

		for _, r := range results {
			sb.WriteString("\n")
			if r.EntityID != "" && r.EntityID != r.FeatureID {
				fmt.Fprintf(&sb, "[%s] feature:%s #%s\n", r.EntityType, r.FeatureID, r.EntityID)
			} else {
				fmt.Fprintf(&sb, "[%s] feature:%s\n", r.EntityType, r.FeatureID)
			}
			fmt.Fprintf(&sb, "  %s: %s\n", r.FieldName, r.Snippet)
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}
