package checkpoint

import "os"

const defaultCLIModel = "haiku"
const defaultAPIModel = "claude-haiku-4-5-20251001"

// Config holds summarizer configuration from environment variables.
type Config struct {
	APIKey    string
	Model     string
	ClaudeBin string // absolute path to claude binary (empty = not available)
}

// LoadConfig reads summarizer configuration from environment variables.
// Prefers the claude CLI (zero-config). Falls back to Anthropic API if
// ANTHROPIC_API_KEY is set. Disabled if neither is available.
func LoadConfig() Config {
	model := os.Getenv("DOCKET_SUMMARIZER_MODEL")

	if v := os.Getenv("DOCKET_SUMMARIZER_ENABLED"); v == "false" {
		return Config{}
	}

	// Prefer CLI — zero config, uses Claude Code's auth
	// Resolve absolute path now so the worker doesn't depend on PATH later
	if bin := FindClaudeBin(); bin != "" {
		if model == "" {
			model = defaultCLIModel
		}
		return Config{Model: model, ClaudeBin: bin}
	}

	// Fall back to direct API if key is set
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey != "" {
		if model == "" {
			model = defaultAPIModel
		}
		return Config{APIKey: apiKey, Model: model}
	}

	return Config{}
}
