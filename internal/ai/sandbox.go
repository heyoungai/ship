package ai

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Sandbox confines file-tool paths to a project root.
type Sandbox struct {
	Root string
}

func NewSandbox(root string) (*Sandbox, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		// Root may not exist yet in odd test setups; Abs is enough.
		abs, err = filepath.Abs(root)
		if err != nil {
			return nil, err
		}
	}
	return &Sandbox{Root: abs}, nil
}

// Resolve returns an absolute path under Root, or an error if it escapes.
func (s *Sandbox) Resolve(rel string) (string, error) {
	if strings.TrimSpace(rel) == "" {
		return "", fmt.Errorf("path is empty")
	}
	cleaned := filepath.Clean(rel)
	var abs string
	if filepath.IsAbs(cleaned) {
		abs = cleaned
	} else {
		abs = filepath.Join(s.Root, cleaned)
	}
	abs = filepath.Clean(abs)

	root := s.Root
	relToRoot, err := filepath.Rel(root, abs)
	if err != nil {
		return "", fmt.Errorf("path outside project root: %s", rel)
	}
	if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path outside project root: %s", rel)
	}
	return abs, nil
}
