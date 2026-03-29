package store

import (
	"fmt"
	"sort"
)

type TemplateSubtask struct {
	Title string
	Items []string
}

var featureTemplates = map[string][]TemplateSubtask{
	"feature": {
		{Title: "Planning", Items: []string{"Define acceptance criteria", "Identify key files"}},
		{Title: "Implementation", Items: []string{"Implement core logic", "Add tests"}},
		{Title: "Polish", Items: []string{"Update documentation", "Final review"}},
	},
	"bugfix": {
		{Title: "Investigation", Items: []string{"Reproduce the bug", "Identify root cause"}},
		{Title: "Fix", Items: []string{"Implement fix", "Add regression test"}},
	},
	"chore": {
		{Title: "Work", Items: []string{"Implement changes", "Verify no regressions"}},
	},
	"spike": {
		{Title: "Research", Items: []string{"Explore approaches", "Document findings"}},
	},
}

func ValidFeatureTypes() []string {
	keys := make([]string, 0, len(featureTemplates))
	for k := range featureTemplates {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (s *Store) ApplyTemplate(featureID, typ string) error {
	tmpl, ok := featureTemplates[typ]
	if !ok {
		return fmt.Errorf("unknown feature type %q (valid: %v)", typ, ValidFeatureTypes())
	}

	for pos, st := range tmpl {
		subtask, err := s.AddSubtask(featureID, st.Title, pos+1)
		if err != nil {
			return fmt.Errorf("add subtask %q: %w", st.Title, err)
		}
		for itemPos, itemTitle := range st.Items {
			if _, err := s.AddTaskItem(subtask.ID, itemTitle, itemPos+1); err != nil {
				return fmt.Errorf("add task item %q: %w", itemTitle, err)
			}
		}
	}
	return nil
}
