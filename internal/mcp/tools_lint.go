package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/sniffle6/claude-docket/internal/store"
)

func lintBoardHandler(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		report, err := s.LintBoard()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if report.Total() == 0 {
			return mcp.NewToolResultText("Board health: all clear"), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Board health: %d issues found\n", report.Total())

		if len(report.Stale) > 0 {
			b.WriteString("\nStale (in_progress, no activity 7+ days):\n")
			for _, f := range report.Stale {
				fmt.Fprintf(&b, "  - %s — %s (%s)\n", f.ID, f.Title, f.Detail)
			}
		}
		if len(report.GateBypasses) > 0 {
			b.WriteString("\nGate bypasses (done with unchecked items):\n")
			for _, f := range report.GateBypasses {
				fmt.Fprintf(&b, "  - %s — %s (%s)\n", f.ID, f.Title, f.Detail)
			}
		}
		if len(report.Empty) > 0 {
			b.WriteString("\nEmpty (3+ days, no activity):\n")
			for _, f := range report.Empty {
				fmt.Fprintf(&b, "  - %s — %s\n", f.ID, f.Title)
			}
		}
		if len(report.StuckDevComplete) > 0 {
			b.WriteString("\nStuck dev_complete (7+ days):\n")
			for _, f := range report.StuckDevComplete {
				fmt.Fprintf(&b, "  - %s — %s (%s)\n", f.ID, f.Title, f.Detail)
			}
		}

		return mcp.NewToolResultText(b.String()), nil
	}
}
