package main

import (
	"github.com/sniffle6/claude-docket/internal/handoff"
	"github.com/sniffle6/claude-docket/internal/store"
)

type HandoffCheckpointData = handoff.CheckpointData

func renderHandoff(data *store.HandoffData, cpData *HandoffCheckpointData) string {
	return handoff.Render(data, cpData)
}

func writeHandoffFile(dir string, data *store.HandoffData) error {
	return handoff.WriteFile(dir, data, nil)
}

func writeHandoffFileWithCheckpoints(dir string, data *store.HandoffData, cpData *HandoffCheckpointData) error {
	return handoff.WriteFile(dir, data, cpData)
}

func extractEnrichmentSections(content string) string {
	return handoff.ExtractEnrichmentSections(content)
}

func cleanStaleHandoffs(dir string, activeIDs map[string]bool) {
	handoff.CleanStale(dir, activeIDs)
}
