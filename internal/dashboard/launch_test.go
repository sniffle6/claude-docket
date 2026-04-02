package dashboard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sniffle6/claude-docket/internal/store"
)

func TestRenderLaunchPrompt(t *testing.T) {
	data := &store.LaunchData{
		Feature: store.Feature{
			ID:          "dashboard-writes",
			Title:       "Dashboard Writes",
			Description: "Add write operations to dashboard",
			Status:      "in_progress",
			LeftOff:     "Finished hook design",
			Notes:       "User wants Warp support",
			KeyFiles:    []string{"cmd/docket/hook.go", "dashboard/index.html"},
		},
		TaskItems: []store.TaskItem{
			{ID: 1, Title: "Update Stop hook"},
			{ID: 2, Title: "Add launch endpoint"},
		},
		Issues: []store.Issue{
			{ID: 1, Description: "Theme toggle broken"},
		},
		Subtasks: []store.Subtask{
			{ID: 1, Title: "Hook changes"},
		},
	}

	result := RenderLaunchPrompt(data)

	checks := []string{
		"# Resuming: Dashboard Writes",
		"in_progress",
		"Finished hook design",
		"Update Stop hook",
		"Add launch endpoint",
		"cmd/docket/hook.go",
		"Theme toggle broken",
		"User wants Warp support",
	}
	for _, check := range checks {
		if !strings.Contains(result, check) {
			t.Errorf("launch prompt missing %q", check)
		}
	}
}

func TestRenderLaunchPrompt_Empty(t *testing.T) {
	data := &store.LaunchData{
		Feature: store.Feature{
			ID:       "empty",
			Title:    "Empty Feature",
			Status:   "planned",
			KeyFiles: []string{},
		},
		TaskItems: []store.TaskItem{},
		Issues:    []store.Issue{},
		Subtasks:  []store.Subtask{},
	}

	result := RenderLaunchPrompt(data)

	if !strings.Contains(result, "# Resuming: Empty Feature") {
		t.Error("missing title")
	}
	// Should not contain section headers for empty sections
	if strings.Contains(result, "## Remaining Tasks") {
		t.Error("should not show Remaining Tasks when empty")
	}
	if strings.Contains(result, "## Open Issues") {
		t.Error("should not show Open Issues when empty")
	}
}

func TestReadLaunchConfig_ProjectLevel(t *testing.T) {
	dir := t.TempDir()
	docketDir := filepath.Join(dir, ".docket")
	os.MkdirAll(docketDir, 0755)
	os.WriteFile(filepath.Join(docketDir, "launch.toml"), []byte(
		"launch = \"wt -w docket-{{feature_id}} cmd /k {{script_path}}\"\nfocus = \"wt -w docket-{{feature_id}}\"\n",
	), 0644)

	cfg := ReadLaunchConfig(dir)
	if cfg.Launch != "wt -w docket-{{feature_id}} cmd /k {{script_path}}" {
		t.Errorf("launch = %q", cfg.Launch)
	}
	if cfg.Focus != "wt -w docket-{{feature_id}}" {
		t.Errorf("focus = %q", cfg.Focus)
	}
}

func TestReadLaunchConfig_GlobalFallback(t *testing.T) {
	projDir := t.TempDir()
	globalDir := t.TempDir()

	os.WriteFile(filepath.Join(globalDir, "launch.toml"), []byte(
		"launch = \"kitty sh {{script_path}}\"\n",
	), 0644)

	cfg := ReadLaunchConfigWithGlobal(projDir, globalDir)
	if cfg.Launch != "kitty sh {{script_path}}" {
		t.Errorf("launch = %q", cfg.Launch)
	}
	if cfg.Focus != "" {
		t.Errorf("focus should be empty, got %q", cfg.Focus)
	}
}

func TestReadLaunchConfig_ProjectOverridesGlobal(t *testing.T) {
	projDir := t.TempDir()
	globalDir := t.TempDir()

	os.MkdirAll(filepath.Join(projDir, ".docket"), 0755)
	os.WriteFile(filepath.Join(projDir, ".docket", "launch.toml"), []byte(
		"launch = \"project-cmd\"\n",
	), 0644)
	os.WriteFile(filepath.Join(globalDir, "launch.toml"), []byte(
		"launch = \"global-cmd\"\n",
	), 0644)

	cfg := ReadLaunchConfigWithGlobal(projDir, globalDir)
	if cfg.Launch != "project-cmd" {
		t.Errorf("project should override global, got %q", cfg.Launch)
	}
}

func TestReadLaunchConfig_NoConfig(t *testing.T) {
	cfg := ReadLaunchConfig(t.TempDir())
	if cfg.Launch != "" || cfg.Focus != "" {
		t.Error("should return empty config when no file exists")
	}
}

func TestShellEscapeWindows(t *testing.T) {
	cases := []struct{ in, want string }{
		{"simple", "simple"},
		{"has space", `"has space"`},
		{`has "quote"`, `"has ""quote"""`},
		{"H:\\claude code\\tools", `"H:\claude code\tools"`},
	}
	for _, c := range cases {
		got := ShellEscape(c.in, "windows")
		if got != c.want {
			t.Errorf("ShellEscape(%q, windows) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestShellEscapeUnix(t *testing.T) {
	cases := []struct{ in, want string }{
		{"simple", "simple"},
		{"has space", "'has space'"},
		{"it's here", "'it'\\''s here'"},
		{"/home/user/my project", "'/home/user/my project'"},
	}
	for _, c := range cases {
		got := ShellEscape(c.in, "unix")
		if got != c.want {
			t.Errorf("ShellEscape(%q, unix) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSubstituteLaunchCmd_EscapesVars(t *testing.T) {
	tmpl := "wt -w docket-{{feature_id}} --title {{feature_title}} cmd /k {{script_path}}"
	result := SubstituteTemplate(tmpl, TemplateVars{
		FeatureID:    "my-feature",
		FeatureTitle: "My Feature Title",
		ScriptPath:   `H:\claude code\.docket\launch\my-feature.cmd`,
		ProjectDir:   `H:\claude code`,
	}, "windows")
	if !strings.Contains(result, "docket-my-feature") {
		t.Error("feature_id should be unescaped")
	}
	if !strings.Contains(result, `"My Feature Title"`) {
		t.Errorf("feature_title should be quoted, got %q", result)
	}
	if !strings.Contains(result, `"H:\claude code\.docket\launch\my-feature.cmd"`) {
		t.Errorf("script_path should be quoted, got %q", result)
	}
}
