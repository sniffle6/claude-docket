package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/sniffle6/claude-docket/internal/handoff"
	"github.com/sniffle6/claude-docket/internal/store"
	"github.com/sniffle6/claude-docket/internal/transcript"
)

func checkpointHandler(s *store.Store, projectDir string, onCheckpoint func()) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		endSession := false
		if v, ok := args["end_session"]; ok {
			if b, ok := v.(bool); ok {
				endSession = b
			}
		}

		// MCP tools don't receive a claude_session_id, so we use the generic
		// query. Low risk: one MCP server per project, so typically one session.
		ws, err := s.GetActiveWorkSession()
		if err != nil {
			return mcp.NewToolResultError("no active work session — nothing to checkpoint"), nil
		}

		// Read transcript offset
		offsetPath := filepath.Join(projectDir, ".docket", "transcript-offset")
		var startOffset int64
		if data, err := os.ReadFile(offsetPath); err == nil {
			fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &startOffset)
		}

		// Find transcript path
		transcriptPath := findTranscriptPath(ws.ClaudeSessionID)

		var delta *transcript.Delta
		if transcriptPath != "" {
			var parseErr error
			delta, parseErr = transcript.Parse(transcriptPath, startOffset)
			if parseErr != nil {
				return mcp.NewToolResultError(fmt.Sprintf("parse transcript: %v", parseErr)), nil
			}
		} else {
			delta = &transcript.Delta{EndOffset: startOffset}
		}

		reason := "manual_checkpoint"
		if endSession {
			reason = "manual_end_session"
		}

		job, err := s.EnqueueCheckpointJob(store.CheckpointJobInput{
			WorkSessionID:         ws.ID,
			FeatureID:             ws.FeatureID,
			Reason:                reason,
			TriggerType:           "manual",
			TranscriptStartOffset: startOffset,
			TranscriptEndOffset:   delta.EndOffset,
			SemanticText:          delta.SemanticText,
			MechanicalFacts:       delta.MechanicalFacts,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("enqueue checkpoint: %v", err)), nil
		}
		if onCheckpoint != nil {
			onCheckpoint()
		}

		// Update offset
		os.WriteFile(offsetPath, []byte(fmt.Sprintf("%d", delta.EndOffset)), 0644)

		if endSession {
			data, err := s.GetHandoffData(ws.FeatureID)
			if err == nil {
				handoff.WriteFile(projectDir, data, nil)
			}
			s.CloseWorkSession(ws.ID)

			return mcp.NewToolResultText(fmt.Sprintf(
				"Work session closed for feature %q. Checkpoint #%d enqueued. Handoff written.",
				ws.FeatureID, job.ID,
			)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf(
			"Checkpoint #%d enqueued for feature %q. %d chars semantic text, %d files edited.",
			job.ID, ws.FeatureID, len(delta.SemanticText), len(delta.MechanicalFacts.FilesEdited),
		)), nil
	}
}

func findTranscriptPath(claudeSessionID string) string {
	if claudeSessionID == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	projectsDir := filepath.Join(home, ".claude", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(projectsDir, entry.Name(), claudeSessionID+".jsonl")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

