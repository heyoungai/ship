package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/heyoungai/ship/internal"
)

const (
	maxBashOutput  = 64 * 1024
	maxReadBytes   = 256 * 1024
	bashTimeout    = 60 * time.Second
	maxGrepMatches = 50
	maxFindMatches = 200
)

// ToolEnv is shared state for tool execution.
type ToolEnv struct {
	Sandbox *Sandbox
	DryRun  bool
	Yes     bool
	// ConfirmWrite is called before writing/editing ship.toml when !Yes && !DryRun.
	// If nil, writes proceed (caller may set Yes).
	ConfirmWrite func(path string) (bool, error)
	// OnShipTOMLWritten is optional; after a successful ship.toml write/edit, validate.
	OnShipTOMLWritten func(path string) string
}

// ToolDefs returns OpenAI tool definitions for the advisor.
func ToolDefs() []ToolDef {
	return []ToolDef{
		{
			Type: "function",
			Function: ToolFunctionDef{
				Name:        "read",
				Description: "Read a file under the project root. Optional offset/limit are 1-based line numbers.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path":   map[string]any{"type": "string"},
						"offset": map[string]any{"type": "integer", "description": "1-based start line"},
						"limit":  map[string]any{"type": "integer", "description": "max lines to return"},
					},
					"required": []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunctionDef{
				Name:        "write",
				Description: "Write or overwrite a file under the project root.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path":    map[string]any{"type": "string"},
						"content": map[string]any{"type": "string"},
					},
					"required": []string{"path", "content"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunctionDef{
				Name:        "edit",
				Description: "Replace an exact string occurrence in a file. Fails if old_string is missing or not unique when replace_all is false.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path":        map[string]any{"type": "string"},
						"old_string":  map[string]any{"type": "string"},
						"new_string":  map[string]any{"type": "string"},
						"replace_all": map[string]any{"type": "boolean"},
					},
					"required": []string{"path", "old_string", "new_string"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunctionDef{
				Name:        "bash",
				Description: "Run a shell command in the project root. ship deploy/run/push/rollback are blocked.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{"type": "string"},
					},
					"required": []string{"command"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunctionDef{
				Name:        "grep",
				Description: "Search file contents under the project root for a regex pattern.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"pattern": map[string]any{"type": "string"},
						"path":    map[string]any{"type": "string", "description": "file or directory (default .)"},
						"glob":    map[string]any{"type": "string", "description": "optional glob filter e.g. *.go"},
					},
					"required": []string{"pattern"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunctionDef{
				Name:        "find",
				Description: "List files under a directory matching an optional glob (filepath.Match on base name).",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{"type": "string", "description": "directory (default .)"},
						"glob": map[string]any{"type": "string", "description": "e.g. *.toml or Dockerfile"},
					},
					"required": []string{},
				},
			},
		},
	}
}

// ExecTool runs a named tool and returns a string result for the model.
func ExecTool(ctx context.Context, env *ToolEnv, name string, argsJSON string) string {
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("error: invalid tool args JSON: %v", err)
	}
	switch name {
	case "read":
		return toolRead(env, args)
	case "write":
		return toolWrite(env, args)
	case "edit":
		return toolEdit(env, args)
	case "bash":
		return toolBash(ctx, env, args)
	case "grep":
		return toolGrep(env, args)
	case "find":
		return toolFind(env, args)
	default:
		return fmt.Sprintf("error: unknown tool %q", name)
	}
}

func argString(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func argInt(args map[string]any, key string) int {
	v, ok := args[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case json.Number:
		i, _ := t.Int64()
		return int(i)
	default:
		return 0
	}
}

func argBool(args map[string]any, key string) bool {
	v, ok := args[key]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	default:
		return false
	}
}

func toolRead(env *ToolEnv, args map[string]any) string {
	path, err := env.Sandbox.Resolve(argString(args, "path"))
	if err != nil {
		return "error: " + err.Error()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "error: " + err.Error()
	}
	if len(data) > maxReadBytes {
		data = data[:maxReadBytes]
	}
	content := string(data)
	offset := argInt(args, "offset")
	limit := argInt(args, "limit")
	if offset > 0 || limit > 0 {
		lines := strings.Split(content, "\n")
		start := 0
		if offset > 0 {
			start = offset - 1
			if start > len(lines) {
				start = len(lines)
			}
		}
		end := len(lines)
		if limit > 0 && start+limit < end {
			end = start + limit
		}
		var b strings.Builder
		for i := start; i < end; i++ {
			fmt.Fprintf(&b, "%d|%s\n", i+1, lines[i])
		}
		return b.String()
	}
	return content
}

func isShipTOML(path string) bool {
	return strings.EqualFold(filepath.Base(path), "ship.toml")
}

func maybeConfirmWrite(env *ToolEnv, path string) string {
	if env.DryRun || env.Yes || env.ConfirmWrite == nil || !isShipTOML(path) {
		return ""
	}
	ok, err := env.ConfirmWrite(path)
	if err != nil {
		return "error: " + err.Error()
	}
	if !ok {
		return "error: write cancelled by user"
	}
	return ""
}

func afterShipTOML(env *ToolEnv, path string) string {
	if !isShipTOML(path) {
		return ""
	}
	if env.OnShipTOMLWritten != nil {
		if msg := env.OnShipTOMLWritten(path); msg != "" {
			return "\n" + msg
		}
	}
	// Default: try validate via LoadConfigFrom parent dir.
	dir := filepath.Dir(path)
	if _, err := internal.LoadConfigFrom(dir, ""); err != nil {
		return "\nvalidation: " + err.Error()
	}
	return "\nvalidation: ok"
}

func toolWrite(env *ToolEnv, args map[string]any) string {
	rel := argString(args, "path")
	path, err := env.Sandbox.Resolve(rel)
	if err != nil {
		return "error: " + err.Error()
	}
	content := argString(args, "content")
	if env.DryRun {
		preview := content
		if len(preview) > 4000 {
			preview = preview[:4000] + "\n... (truncated)"
		}
		return fmt.Sprintf("dry-run: would write %s (%d bytes):\n%s", rel, len(content), preview)
	}
	if msg := maybeConfirmWrite(env, path); msg != "" {
		return msg
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "error: " + err.Error()
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "error: " + err.Error()
	}
	return "wrote " + rel + afterShipTOML(env, path)
}

func toolEdit(env *ToolEnv, args map[string]any) string {
	rel := argString(args, "path")
	path, err := env.Sandbox.Resolve(rel)
	if err != nil {
		return "error: " + err.Error()
	}
	oldS := argString(args, "old_string")
	newS := argString(args, "new_string")
	replaceAll := argBool(args, "replace_all")
	data, err := os.ReadFile(path)
	if err != nil {
		return "error: " + err.Error()
	}
	src := string(data)
	if oldS == "" {
		return "error: old_string is empty"
	}
	count := strings.Count(src, oldS)
	if count == 0 {
		return "error: old_string not found"
	}
	if !replaceAll && count > 1 {
		return fmt.Sprintf("error: old_string found %d times; set replace_all=true or make old_string unique", count)
	}
	var out string
	if replaceAll {
		out = strings.ReplaceAll(src, oldS, newS)
	} else {
		out = strings.Replace(src, oldS, newS, 1)
	}
	if env.DryRun {
		return fmt.Sprintf("dry-run: would edit %s (%d replacement(s))", rel, count)
	}
	if msg := maybeConfirmWrite(env, path); msg != "" {
		return msg
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return "error: " + err.Error()
	}
	return fmt.Sprintf("edited %s (%d replacement(s))", rel, count) + afterShipTOML(env, path)
}

func toolBash(ctx context.Context, env *ToolEnv, args map[string]any) string {
	command := argString(args, "command")
	if err := GuardBash(command); err != nil {
		return "error: " + err.Error()
	}
	ctx, cancel := context.WithTimeout(ctx, bashTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	cmd.Dir = env.Sandbox.Root
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := buf.String()
	if len(out) > maxBashOutput {
		out = out[:maxBashOutput] + "\n... (truncated)"
	}
	if err != nil {
		if out == "" {
			return "error: " + err.Error()
		}
		return fmt.Sprintf("%s\nerror: %v", out, err)
	}
	if out == "" {
		return "(no output)"
	}
	return out
}

func toolGrep(env *ToolEnv, args map[string]any) string {
	pattern := argString(args, "pattern")
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "error: invalid regex: " + err.Error()
	}
	rootRel := argString(args, "path")
	if rootRel == "" {
		rootRel = "."
	}
	root, err := env.Sandbox.Resolve(rootRel)
	if err != nil {
		return "error: " + err.Error()
	}
	globPat := argString(args, "glob")

	var matches []string
	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == ".ship" {
				return filepath.SkipDir
			}
			return nil
		}
		if globPat != "" {
			ok, _ := filepath.Match(globPat, d.Name())
			if !ok {
				return nil
			}
		}
		data, err := os.ReadFile(path)
		if err != nil || len(data) > maxReadBytes {
			return nil
		}
		if !re.Match(data) && !bytes.Contains(data, []byte(pattern)) {
			// still line-scan with regex
		}
		rel, _ := filepath.Rel(env.Sandbox.Root, path)
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if re.MatchString(line) {
				matches = append(matches, fmt.Sprintf("%s:%d:%s", filepath.ToSlash(rel), i+1, line))
				if len(matches) >= maxGrepMatches {
					return io.EOF
				}
			}
		}
		return nil
	})
	if walkErr != nil && walkErr != io.EOF {
		return "error: " + walkErr.Error()
	}
	if len(matches) == 0 {
		return "(no matches)"
	}
	out := strings.Join(matches, "\n")
	if len(matches) >= maxGrepMatches {
		out += "\n... (truncated)"
	}
	return out
}

func toolFind(env *ToolEnv, args map[string]any) string {
	rootRel := argString(args, "path")
	if rootRel == "" {
		rootRel = "."
	}
	root, err := env.Sandbox.Resolve(rootRel)
	if err != nil {
		return "error: " + err.Error()
	}
	globPat := argString(args, "glob")
	var found []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == ".ship" {
				return filepath.SkipDir
			}
			return nil
		}
		if globPat != "" {
			ok, _ := filepath.Match(globPat, d.Name())
			if !ok {
				return nil
			}
		}
		rel, _ := filepath.Rel(env.Sandbox.Root, path)
		found = append(found, filepath.ToSlash(rel))
		if len(found) >= maxFindMatches {
			return io.EOF
		}
		return nil
	})
	if len(found) == 0 {
		return "(no files)"
	}
	out := strings.Join(found, "\n")
	if len(found) >= maxFindMatches {
		out += "\n... (truncated)"
	}
	return out
}
