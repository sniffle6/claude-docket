package store

// LaunchData holds everything needed to generate a launch prompt file.
type LaunchData struct {
	Feature   Feature
	Subtasks  []Subtask  // active only
	TaskItems []TaskItem // unchecked only
	Issues    []Issue    // open only
}

// GetLaunchData gathers feature details, unchecked tasks, and open issues
// in one call for the launch endpoint.
func (s *Store) GetLaunchData(featureID string) (*LaunchData, error) {
	f, err := s.GetFeature(featureID)
	if err != nil {
		return nil, err
	}

	subtasks, _ := s.GetSubtasksForFeature(featureID, false)
	if subtasks == nil {
		subtasks = []Subtask{}
	}

	var uncheckedItems []TaskItem
	for _, st := range subtasks {
		for _, item := range st.Items {
			if !item.Checked {
				uncheckedItems = append(uncheckedItems, item)
			}
		}
	}
	if uncheckedItems == nil {
		uncheckedItems = []TaskItem{}
	}

	issues, _ := s.GetIssuesForFeature(featureID)
	var openIssues []Issue
	for _, issue := range issues {
		if issue.Status == "open" {
			openIssues = append(openIssues, issue)
		}
	}
	if openIssues == nil {
		openIssues = []Issue{}
	}

	return &LaunchData{
		Feature:   *f,
		Subtasks:  subtasks,
		TaskItems: uncheckedItems,
		Issues:    openIssues,
	}, nil
}

type HandoffSubtask struct {
	Title string
	Done  int
	Total int
}

type HandoffData struct {
	Feature        Feature
	Done           int
	Total          int
	NextTasks      []string // up to 3 uncompleted task item titles
	SubtaskSummary []HandoffSubtask
	RecentSessions []Session // last 3
}

func (s *Store) GetHandoffData(featureID string) (*HandoffData, error) {
	f, err := s.GetFeature(featureID)
	if err != nil {
		return nil, err
	}

	done, total, _ := s.GetFeatureProgress(featureID)

	subtasks, _ := s.GetSubtasksForFeature(featureID, false)
	var nextTasks []string
	var subtaskSummary []HandoffSubtask
	for _, st := range subtasks {
		stDone := 0
		for _, item := range st.Items {
			if item.Checked {
				stDone++
			} else if len(nextTasks) < 3 {
				nextTasks = append(nextTasks, item.Title)
			}
		}
		subtaskSummary = append(subtaskSummary, HandoffSubtask{
			Title: st.Title,
			Done:  stDone,
			Total: len(st.Items),
		})
	}

	rows, err := s.db.Query(
		`SELECT id, COALESCE(feature_id, ''), summary, files_touched, commits, auto_linked, link_reason, created_at FROM sessions WHERE feature_id = ? ORDER BY created_at DESC LIMIT 3`,
		featureID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sessions, _ := scanSessions(rows)

	return &HandoffData{
		Feature:        *f,
		Done:           done,
		Total:          total,
		NextTasks:      nextTasks,
		SubtaskSummary: subtaskSummary,
		RecentSessions: sessions,
	}, nil
}
