package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type ImportResult struct {
	SubtaskCount  int
	TaskItemCount int
	Subtasks      []ImportedSubtask
}

type ImportedSubtask struct {
	Title    string
	ItemIDs  []int64
	Position int
}

var (
	taskHeadingRe = regexp.MustCompile(`^### Task (\d+): (.+)$`)
	stepRe        = regexp.MustCompile(`^- \[ \] \*\*Step \d+: (.+)\*\*$`)
	fileEntryRe   = regexp.MustCompile("^- (?:Create|Modify|Test): `(.+)`$")
)

func (s *Store) ImportPlan(featureID, planPath string) (*ImportResult, error) {
	feat, err := s.GetFeature(featureID)
	if err != nil {
		return nil, fmt.Errorf("get feature: %w", err)
	}
	if feat.Status == "done" {
		return nil, fmt.Errorf("feature %q has status done — reset status before re-importing a plan", featureID)
	}

	file, err := os.Open(planPath)
	if err != nil {
		return nil, fmt.Errorf("open plan: %w", err)
	}
	defer file.Close()

	if err := s.ArchiveSubtasks(featureID); err != nil {
		return nil, fmt.Errorf("archive existing: %w", err)
	}

	result := &ImportResult{}
	scanner := bufio.NewScanner(file)

	var currentSubtaskID int64
	var currentFiles []string
	var inFilesSection bool
	position := 0

	for scanner.Scan() {
		line := scanner.Text()

		if m := taskHeadingRe.FindStringSubmatch(line); m != nil {
			position++
			title := "Task " + m[1] + ": " + m[2]
			st, err := s.AddSubtask(featureID, title, position)
			if err != nil {
				return nil, fmt.Errorf("add subtask %q: %w", title, err)
			}
			currentSubtaskID = st.ID
			currentFiles = nil
			inFilesSection = false
			result.SubtaskCount++
			result.Subtasks = append(result.Subtasks, ImportedSubtask{
				Title:    title,
				Position: position,
			})
			continue
		}

		if strings.HasPrefix(line, "**Files:**") {
			inFilesSection = true
			currentFiles = nil
			continue
		}

		if inFilesSection {
			if m := fileEntryRe.FindStringSubmatch(line); m != nil {
				currentFiles = append(currentFiles, m[1])
				continue
			}
			if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "-") {
				inFilesSection = false
			}
		}

		if currentSubtaskID > 0 {
			if m := stepRe.FindStringSubmatch(line); m != nil {
				itemPosition := len(result.Subtasks[len(result.Subtasks)-1].ItemIDs) + 1
				item, err := s.AddTaskItem(currentSubtaskID, m[1], itemPosition)
				if err != nil {
					return nil, fmt.Errorf("add task item: %w", err)
				}
				if len(currentFiles) > 0 {
					kf, _ := json.Marshal(currentFiles)
					s.db.Exec(`UPDATE task_items SET key_files = ? WHERE id = ?`, string(kf), item.ID)
				}
				result.TaskItemCount++
				result.Subtasks[len(result.Subtasks)-1].ItemIDs = append(
					result.Subtasks[len(result.Subtasks)-1].ItemIDs, item.ID,
				)
				continue
			}
		}
	}

	return result, nil
}
