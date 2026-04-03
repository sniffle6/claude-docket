package store

import (
	"fmt"
	"strings"
)

// SearchOpts controls filtering and output for search queries.
type SearchOpts struct {
	Scope     []string // entity types to include (nil = all)
	FeatureID string   // limit to one feature (empty = all)
	Verbose   bool     // full text vs snippets
	Limit     int      // max results (0 = default 20)
}

// SearchResult is a single match from the FTS5 search index.
type SearchResult struct {
	EntityType string  `json:"entity_type"`
	EntityID   string  `json:"entity_id"`
	FeatureID  string  `json:"feature_id"`
	FieldName  string  `json:"field_name"`
	Snippet    string  `json:"snippet"`
	Rank       float64 `json:"rank"`
}

// Search queries the FTS5 search index and returns ranked results.
func (s *Store) Search(query string, opts SearchOpts) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}

	// Build the content column expression: snippet or raw content
	contentExpr := "snippet(search_index, 4, '[', ']', '...', 12)"
	if opts.Verbose {
		contentExpr = "content"
	}

	// Base query with FTS5 MATCH
	q := fmt.Sprintf(
		`SELECT entity_type, entity_id, feature_id, field_name, %s, rank
		 FROM search_index
		 WHERE search_index MATCH ?`,
		contentExpr,
	)
	args := []interface{}{query}

	// Optional filters
	if len(opts.Scope) > 0 {
		placeholders := make([]string, len(opts.Scope))
		for i, sc := range opts.Scope {
			placeholders[i] = "?"
			args = append(args, sc)
		}
		q += " AND entity_type IN (" + strings.Join(placeholders, ",") + ")"
	}
	if opts.FeatureID != "" {
		q += " AND feature_id = ?"
		args = append(args, opts.FeatureID)
	}

	q += " ORDER BY rank LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("search query: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.EntityType, &r.EntityID, &r.FeatureID, &r.FieldName, &r.Snippet, &r.Rank); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// RebuildSearchIndex drops and repopulates the FTS5 index from source tables.
func (s *Store) RebuildSearchIndex() error {
	if _, err := s.db.Exec("DELETE FROM search_index"); err != nil {
		return fmt.Errorf("clear search index: %w", err)
	}

	populations := []string{
		`INSERT INTO search_index(entity_type, entity_id, feature_id, field_name, content)
		 SELECT 'feature', id, id, 'title', title FROM features WHERE title != ''
		 UNION ALL SELECT 'feature', id, id, 'description', description FROM features WHERE description != ''
		 UNION ALL SELECT 'feature', id, id, 'left_off', left_off FROM features WHERE left_off != ''
		 UNION ALL SELECT 'feature', id, id, 'notes', notes FROM features WHERE notes != ''
		 UNION ALL SELECT 'feature', id, id, 'key_files', key_files FROM features WHERE key_files != '[]'
		 UNION ALL SELECT 'feature', id, id, 'tags', tags FROM features WHERE tags != '[]'`,

		`INSERT INTO search_index(entity_type, entity_id, feature_id, field_name, content)
		 SELECT 'decision', CAST(id AS TEXT), feature_id, 'approach', approach FROM decisions WHERE approach != ''
		 UNION ALL SELECT 'decision', CAST(id AS TEXT), feature_id, 'reason', reason FROM decisions WHERE reason != ''`,

		`INSERT INTO search_index(entity_type, entity_id, feature_id, field_name, content)
		 SELECT 'issue', CAST(id AS TEXT), feature_id, 'description', description FROM issues WHERE description != ''`,

		`INSERT INTO search_index(entity_type, entity_id, feature_id, field_name, content)
		 SELECT 'note', CAST(id AS TEXT), feature_id, 'content', content FROM notes WHERE content != ''`,

		`INSERT INTO search_index(entity_type, entity_id, feature_id, field_name, content)
		 SELECT 'session', CAST(id AS TEXT), COALESCE(feature_id, ''), 'summary', summary FROM sessions WHERE summary != ''`,

		`INSERT INTO search_index(entity_type, entity_id, feature_id, field_name, content)
		 SELECT 'subtask', CAST(id AS TEXT), feature_id, 'title', title FROM subtasks WHERE title != ''`,

		`INSERT INTO search_index(entity_type, entity_id, feature_id, field_name, content)
		 SELECT 'task_item', CAST(ti.id AS TEXT), s.feature_id, 'title', ti.title
		 FROM task_items ti JOIN subtasks s ON s.id = ti.subtask_id WHERE ti.title != ''`,

		`INSERT INTO search_index(entity_type, entity_id, feature_id, field_name, content)
		 SELECT 'task_item', CAST(ti.id AS TEXT), s.feature_id, 'outcome', ti.outcome
		 FROM task_items ti JOIN subtasks s ON s.id = ti.subtask_id WHERE ti.outcome != ''`,

		`INSERT INTO search_index(entity_type, entity_id, feature_id, field_name, content)
		 SELECT 'observation', CAST(id AS TEXT), feature_id, 'summary_text', summary_text
		 FROM checkpoint_observations WHERE summary_text != ''`,
	}

	for _, sql := range populations {
		if _, err := s.db.Exec(sql); err != nil {
			return fmt.Errorf("populate search index: %w", err)
		}
	}
	return nil
}
