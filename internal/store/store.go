package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Feature struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Status       string    `json:"status"`
	LeftOff      string    `json:"left_off"`
	KeyFiles     []string  `json:"key_files"`
	WorktreePath string    `json:"worktree_path"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type FeatureUpdate struct {
	Title        *string   `json:"title,omitempty"`
	Description  *string   `json:"description,omitempty"`
	Status       *string   `json:"status,omitempty"`
	LeftOff      *string   `json:"left_off,omitempty"`
	KeyFiles     *[]string `json:"key_files,omitempty"`
	WorktreePath *string   `json:"worktree_path,omitempty"`
}

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = nonAlphaNum.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

type Store struct {
	db *sql.DB
}

func Open(projectDir string) (*Store, error) {
	featDir := filepath.Join(projectDir, ".feat")
	if err := os.MkdirAll(featDir, 0755); err != nil {
		return nil, fmt.Errorf("create .feat dir: %w", err)
	}

	dbPath := filepath.Join(featDir, "features.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA foreign_keys=ON")

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) AddFeature(title, description string) (*Feature, error) {
	id := slugify(title)
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`INSERT INTO features (id, title, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		id, title, description, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert feature: %w", err)
	}
	return s.GetFeature(id)
}

func (s *Store) GetFeature(id string) (*Feature, error) {
	row := s.db.QueryRow(
		`SELECT id, title, description, status, left_off, key_files, worktree_path, created_at, updated_at FROM features WHERE id = ?`,
		id,
	)
	var f Feature
	var keyFilesJSON string
	err := row.Scan(&f.ID, &f.Title, &f.Description, &f.Status, &f.LeftOff, &keyFilesJSON, &f.WorktreePath, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get feature %q: %w", id, err)
	}
	json.Unmarshal([]byte(keyFilesJSON), &f.KeyFiles)
	if f.KeyFiles == nil {
		f.KeyFiles = []string{}
	}
	return &f, nil
}

func (s *Store) UpdateFeature(id string, u FeatureUpdate) error {
	sets := []string{}
	args := []any{}
	if u.Title != nil {
		sets = append(sets, "title = ?")
		args = append(args, *u.Title)
	}
	if u.Description != nil {
		sets = append(sets, "description = ?")
		args = append(args, *u.Description)
	}
	if u.Status != nil {
		sets = append(sets, "status = ?")
		args = append(args, *u.Status)
	}
	if u.LeftOff != nil {
		sets = append(sets, "left_off = ?")
		args = append(args, *u.LeftOff)
	}
	if u.KeyFiles != nil {
		kf, _ := json.Marshal(*u.KeyFiles)
		sets = append(sets, "key_files = ?")
		args = append(args, string(kf))
	}
	if u.WorktreePath != nil {
		sets = append(sets, "worktree_path = ?")
		args = append(args, *u.WorktreePath)
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, time.Now().UTC())
	args = append(args, id)
	query := fmt.Sprintf("UPDATE features SET %s WHERE id = ?", strings.Join(sets, ", "))
	res, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update feature: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("feature %q not found", id)
	}
	return nil
}

func (s *Store) ListFeatures(status string) ([]Feature, error) {
	query := `SELECT id, title, description, status, left_off, key_files, worktree_path, created_at, updated_at FROM features`
	var args []any
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY updated_at DESC"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list features: %w", err)
	}
	defer rows.Close()
	var features []Feature
	for rows.Next() {
		var f Feature
		var keyFilesJSON string
		if err := rows.Scan(&f.ID, &f.Title, &f.Description, &f.Status, &f.LeftOff, &keyFilesJSON, &f.WorktreePath, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan feature: %w", err)
		}
		json.Unmarshal([]byte(keyFilesJSON), &f.KeyFiles)
		if f.KeyFiles == nil {
			f.KeyFiles = []string{}
		}
		features = append(features, f)
	}
	return features, nil
}
