package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const maxAgentsMDBytes = 8 * 1024

// LoadAgentsMD reads AGENTS.md from root if present. Empty string if missing.
// Content is truncated to keep the system context small (pi-style).
func LoadAgentsMD(root string) (string, error) {
	path := filepath.Join(root, "AGENTS.md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	s := strings.TrimSpace(string(data))
	if s == "" {
		return "", nil
	}
	if len(data) > maxAgentsMDBytes {
		// Truncate on a rune boundary when possible.
		s = string(data[:maxAgentsMDBytes])
		for !utf8.ValidString(s) && len(s) > 0 {
			s = s[:len(s)-1]
		}
		s = strings.TrimSpace(s) + "\n… (AGENTS.md truncated)"
	}
	return s, nil
}

// SystemWithAgents appends project instructions to the base system prompt.
func SystemWithAgents(base, agents string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = DefaultSystemPrompt
	}
	agents = strings.TrimSpace(agents)
	if agents == "" {
		return base
	}
	return fmt.Sprintf("%s\n\nProject instructions from AGENTS.md:\n%s", base, agents)
}
