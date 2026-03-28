package store

import (
	"encoding/json"
	"fmt"
	"time"
)

type Subtask struct {
	ID         int64      `json:"id"`
	FeatureID  string     `json:"feature_id"`
	Title      string     `json:"title"`
	Position   int        `json:"position"`
	Archived   bool       `json:"archived"`
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	Items      []TaskItem `json:"items,omitempty"`
}

type TaskItem struct {
	ID         int64     `json:"id"`
	SubtaskID  int64     `json:"subtask_id"`
	Title      string    `json:"title"`
	Checked    bool      `json:"checked"`
	KeyFiles   []string  `json:"key_files"`
	Outcome    string    `json:"outcome,omitempty"`
	CommitHash string    `json:"commit_hash,omitempty"`
	Position   int       `json:"position"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type TaskItemCompletion struct {
	Outcome    string
	CommitHash string
	KeyFiles   []string
}

func (s *Store) AddSubtask(featureID, title string, position int) (*Subtask, error) {
	res, err := s.db.Exec(
		`INSERT INTO subtasks (feature_id, title, position) VALUES (?, ?, ?)`,
		featureID, title, position,
	)
	if err != nil {
		return nil, fmt.Errorf("insert subtask: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetSubtask(id)
}

func (s *Store) GetSubtask(id int64) (*Subtask, error) {
	var st Subtask
	var archivedAt *time.Time
	err := s.db.QueryRow(
		`SELECT id, feature_id, title, position, archived, archived_at, created_at FROM subtasks WHERE id = ?`, id,
	).Scan(&st.ID, &st.FeatureID, &st.Title, &st.Position, &st.Archived, &archivedAt, &st.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get subtask %d: %w", id, err)
	}
	st.ArchivedAt = archivedAt
	return &st, nil
}

func (s *Store) AddTaskItem(subtaskID int64, title string, position int) (*TaskItem, error) {
	now := time.Now().UTC()
	res, err := s.db.Exec(
		`INSERT INTO task_items (subtask_id, title, position, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		subtaskID, title, position, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert task item: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetTaskItem(id)
}

func (s *Store) GetTaskItem(id int64) (*TaskItem, error) {
	var item TaskItem
	var keyFilesJSON string
	err := s.db.QueryRow(
		`SELECT id, subtask_id, title, checked, key_files, outcome, commit_hash, position, created_at, updated_at FROM task_items WHERE id = ?`, id,
	).Scan(&item.ID, &item.SubtaskID, &item.Title, &item.Checked, &keyFilesJSON, &item.Outcome, &item.CommitHash, &item.Position, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get task item %d: %w", id, err)
	}
	json.Unmarshal([]byte(keyFilesJSON), &item.KeyFiles)
	if item.KeyFiles == nil {
		item.KeyFiles = []string{}
	}
	return &item, nil
}

func (s *Store) CompleteTaskItem(id int64, c TaskItemCompletion) error {
	kf, _ := json.Marshal(c.KeyFiles)
	_, err := s.db.Exec(
		`UPDATE task_items SET checked = 1, outcome = ?, commit_hash = ?, key_files = ?, updated_at = ? WHERE id = ?`,
		c.Outcome, c.CommitHash, string(kf), time.Now().UTC(), id,
	)
	return err
}

func (s *Store) GetSubtasksForFeature(featureID string, includeArchived bool) ([]Subtask, error) {
	query := `SELECT id, feature_id, title, position, archived, archived_at, created_at FROM subtasks WHERE feature_id = ?`
	if !includeArchived {
		query += " AND archived = 0"
	}
	query += " ORDER BY archived ASC, position ASC"

	rows, err := s.db.Query(query, featureID)
	if err != nil {
		return nil, fmt.Errorf("get subtasks: %w", err)
	}
	defer rows.Close()

	var subtasks []Subtask
	for rows.Next() {
		var st Subtask
		var archivedAt *time.Time
		if err := rows.Scan(&st.ID, &st.FeatureID, &st.Title, &st.Position, &st.Archived, &archivedAt, &st.CreatedAt); err != nil {
			return nil, err
		}
		st.ArchivedAt = archivedAt
		items, err := s.GetTaskItemsForSubtask(st.ID)
		if err != nil {
			return nil, err
		}
		st.Items = items
		subtasks = append(subtasks, st)
	}
	return subtasks, nil
}

func (s *Store) GetTaskItemsForSubtask(subtaskID int64) ([]TaskItem, error) {
	rows, err := s.db.Query(
		`SELECT id, subtask_id, title, checked, key_files, outcome, commit_hash, position, created_at, updated_at FROM task_items WHERE subtask_id = ? ORDER BY position ASC`,
		subtaskID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []TaskItem
	for rows.Next() {
		var item TaskItem
		var keyFilesJSON string
		if err := rows.Scan(&item.ID, &item.SubtaskID, &item.Title, &item.Checked, &keyFilesJSON, &item.Outcome, &item.CommitHash, &item.Position, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(keyFilesJSON), &item.KeyFiles)
		if item.KeyFiles == nil {
			item.KeyFiles = []string{}
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Store) ArchiveSubtasks(featureID string) error {
	_, err := s.db.Exec(
		`UPDATE subtasks SET archived = 1, archived_at = ? WHERE feature_id = ? AND archived = 0`,
		time.Now().UTC(), featureID,
	)
	return err
}

func (s *Store) GetFeatureProgress(featureID string) (done int, total int, err error) {
	err = s.db.QueryRow(
		`SELECT COALESCE(SUM(ti.checked), 0), COUNT(ti.id)
		 FROM task_items ti
		 JOIN subtasks st ON ti.subtask_id = st.id
		 WHERE st.feature_id = ? AND st.archived = 0`,
		featureID,
	).Scan(&done, &total)
	return
}
