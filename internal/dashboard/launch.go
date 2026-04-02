package dashboard

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sniffle6/claude-docket/internal/store"
)

// LaunchConfig holds the parsed launch.toml settings.
type LaunchConfig struct {
	Launch string // command template to open a new terminal session
	Focus  string // command template to focus an existing window
}

// TemplateVars holds the values substituted into a launch command template.
type TemplateVars struct {
	FeatureID    string
	FeatureTitle string
	ScriptPath   string
	ProjectDir   string
}

// ReadLaunchConfig reads the launch config for projectDir, falling back to the
// OS user config dir (~/.config/docket or equivalent) when no project-level
// file exists.
func ReadLaunchConfig(projectDir string) LaunchConfig {
	globalDir := ""
	if d, err := os.UserConfigDir(); err == nil {
		globalDir = filepath.Join(d, "docket")
	}
	return ReadLaunchConfigWithGlobal(projectDir, globalDir)
}

// ReadLaunchConfigWithGlobal is the same as ReadLaunchConfig but accepts an
// explicit globalDir — exported for testing.
func ReadLaunchConfigWithGlobal(projectDir, globalDir string) LaunchConfig {
	projectPath := filepath.Join(projectDir, ".docket", "launch.toml")
	if cfg, err := parseLaunchToml(projectPath); err == nil {
		return cfg
	}
	if globalDir != "" {
		globalPath := filepath.Join(globalDir, "launch.toml")
		if cfg, err := parseLaunchToml(globalPath); err == nil {
			return cfg
		}
	}
	return LaunchConfig{}
}

// parseLaunchToml reads a simple TOML file (key = "value" pairs) and returns
// a LaunchConfig. Returns an error when the file cannot be read.
func parseLaunchToml(path string) (LaunchConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return LaunchConfig{}, err
	}
	defer f.Close()

	var cfg LaunchConfig
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := parseTomlLine(line)
		if !ok {
			continue
		}
		switch key {
		case "launch":
			cfg.Launch = value
		case "focus":
			cfg.Focus = value
		}
	}
	return cfg, scanner.Err()
}

// parseTomlLine parses a line of the form: key = "value"
// Returns the key, unescaped value, and true on success.
func parseTomlLine(line string) (key, value string, ok bool) {
	idx := strings.IndexByte(line, '=')
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	rest := strings.TrimSpace(line[idx+1:])
	if len(rest) >= 2 && rest[0] == '"' && rest[len(rest)-1] == '"' {
		// strip surrounding quotes and unescape internal \"
		inner := rest[1 : len(rest)-1]
		value = strings.ReplaceAll(inner, `\"`, `"`)
		return key, value, true
	}
	// bare value (no quotes)
	return key, rest, true
}

// ShellEscape wraps value in platform-appropriate shell quoting only when the
// value contains characters that require it.
//
//   - "windows": wraps in double-quotes, escaping internal " as ""
//   - "unix":    wraps in single-quotes, escaping internal ' as '\''
func ShellEscape(value, platform string) string {
	switch platform {
	case "windows":
		if !strings.ContainsAny(value, " \t\"") {
			return value
		}
		return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
	default: // unix
		if !strings.ContainsAny(value, " \t'\\\"(){}$`!&|;<>?*~") {
			return value
		}
		return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
	}
}

// SubstituteTemplate replaces template variables in tmpl using vars, applying
// ShellEscape on all vars except FeatureID (which is used raw in window names
// and identifiers where quoting would break things).
func SubstituteTemplate(tmpl string, vars TemplateVars, platform string) string {
	r := strings.NewReplacer(
		"{{feature_id}}", vars.FeatureID,
		"{{feature_title}}", ShellEscape(vars.FeatureTitle, platform),
		"{{script_path}}", ShellEscape(vars.ScriptPath, platform),
		"{{project_dir}}", ShellEscape(vars.ProjectDir, platform),
	)
	return r.Replace(tmpl)
}

// SubstituteLaunchCmd is the legacy wrapper kept for callers that predate
// LaunchConfig / SubstituteTemplate. handoffFile maps to script_path.
func SubstituteLaunchCmd(template, handoffFile, featureTitle, featureID, projectDir string) string {
	return SubstituteTemplate(template, TemplateVars{
		FeatureID:    featureID,
		FeatureTitle: featureTitle,
		ScriptPath:   handoffFile,
		ProjectDir:   projectDir,
	}, "")
}

// RenderLaunchPrompt generates a markdown prompt file for launching a new
// Claude session with full feature context.
func RenderLaunchPrompt(data *store.LaunchData) string {
	var b strings.Builder
	f := data.Feature

	fmt.Fprintf(&b, "# Resuming: %s\n\n", f.Title)

	if f.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", f.Description)
	}

	fmt.Fprintf(&b, "**Status:** %s\n", f.Status)
	if f.LeftOff != "" {
		fmt.Fprintf(&b, "**Left off:** %s\n", f.LeftOff)
	}
	b.WriteString("\n")

	if len(data.TaskItems) > 0 {
		b.WriteString("## Remaining Tasks\n")
		for _, item := range data.TaskItems {
			fmt.Fprintf(&b, "- [ ] %s\n", item.Title)
		}
		b.WriteString("\n")
	}

	if len(f.KeyFiles) > 0 {
		b.WriteString("## Key Files\n")
		for _, kf := range f.KeyFiles {
			fmt.Fprintf(&b, "- %s\n", kf)
		}
		b.WriteString("\n")
	}

	if len(data.Issues) > 0 {
		b.WriteString("## Open Issues\n")
		for _, issue := range data.Issues {
			fmt.Fprintf(&b, "- #%d: %s\n", issue.ID, issue.Description)
		}
		b.WriteString("\n")
	}

	if f.Notes != "" {
		b.WriteString("## Notes\n")
		fmt.Fprintf(&b, "%s\n", f.Notes)
	}

	return b.String()
}

// renderLaunchExtras generates additional context sections (unchecked tasks,
// open issues, notes) to append when using an existing handoff file as base.
func renderLaunchExtras(data *store.LaunchData) string {
	var b strings.Builder

	if len(data.TaskItems) > 0 {
		b.WriteString("## Remaining Tasks (current)\n")
		for _, item := range data.TaskItems {
			fmt.Fprintf(&b, "- [ ] %s\n", item.Title)
		}
		b.WriteString("\n")
	}

	if len(data.Issues) > 0 {
		b.WriteString("## Open Issues (current)\n")
		for _, issue := range data.Issues {
			fmt.Fprintf(&b, "- #%d: %s\n", issue.ID, issue.Description)
		}
		b.WriteString("\n")
	}

	if data.Feature.Notes != "" {
		b.WriteString("## Notes\n")
		fmt.Fprintf(&b, "%s\n", data.Feature.Notes)
	}

	return b.String()
}
