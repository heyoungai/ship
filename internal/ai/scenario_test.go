package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewProviderMissingKey(t *testing.T) {
	if _, err := NewProvider("", "", ""); err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestLoadAgentsMD(t *testing.T) {
	dir := t.TempDir()
	got, err := LoadAgentsMD(dir)
	if err != nil || got != "" {
		t.Fatalf("missing file: got %q err %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("  use docker driver  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = LoadAgentsMD(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "use docker driver" {
		t.Fatalf("got %q", got)
	}
	sys := SystemWithAgents(DefaultSystemPrompt, got)
	if !strings.Contains(sys, "AGENTS.md") || !strings.Contains(sys, "use docker driver") {
		t.Fatalf("system missing agents: %s", sys)
	}
}

func TestShipTOMLValidationFeedback(t *testing.T) {
	dir := t.TempDir()
	sb, err := NewSandbox(dir)
	if err != nil {
		t.Fatal(err)
	}
	env := &ToolEnv{Sandbox: sb, Yes: true}

	bad := ExecTool(context.Background(), env, "write", mustJSON(t, map[string]any{
		"path":    "ship.toml",
		"content": "schema = 1\n",
	}))
	if !strings.Contains(bad, "validation:") || !strings.Contains(bad, "schema") {
		t.Fatalf("expected validation error, got %q", bad)
	}

	// Minimal-ish valid-ish content is hard; at least schema=2 with docker fields from example patterns.
	goodBody := `
schema = 2
[project]
name = "demo"
[version]
source = "static"
static = "0.0.1"
fallback = "static"
[features]
publish = false
deploy = false
rollback = false
verify = false
[build]
driver = "docker"
[build.docker]
image = "demo"
dockerfile = "./Dockerfile"
platforms = ["linux/amd64"]
load = true
[publish]
driver = "none"
[deploy]
driver = "none"
`
	// Need a Dockerfile path to exist? Validate may not check file exists.
	good := ExecTool(context.Background(), env, "write", mustJSON(t, map[string]any{
		"path":    "ship.toml",
		"content": goodBody,
	}))
	if !strings.Contains(good, "validation: ok") {
		t.Fatalf("expected validation ok, got %q", good)
	}
}

func TestGrepAndFindScenarios(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte("package main\nconst Answer = 42\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	sb, _ := NewSandbox(dir)
	env := &ToolEnv{Sandbox: sb}

	grepOut := ExecTool(context.Background(), env, "grep", mustJSON(t, map[string]any{
		"pattern": `Answer\s*=`,
		"glob":    "*.go",
	}))
	if !strings.Contains(grepOut, "app.go") || !strings.Contains(grepOut, "42") {
		t.Fatalf("grep: %q", grepOut)
	}

	findOut := ExecTool(context.Background(), env, "find", mustJSON(t, map[string]any{
		"glob": "*.go",
	}))
	if !strings.Contains(findOut, "app.go") {
		t.Fatalf("find: %q", findOut)
	}
	if strings.Contains(findOut, "note.txt") {
		t.Fatalf("find should respect glob: %q", findOut)
	}
}

func TestBashEchoAllowed(t *testing.T) {
	dir := t.TempDir()
	sb, _ := NewSandbox(dir)
	env := &ToolEnv{Sandbox: sb}
	out := ExecTool(context.Background(), env, "bash", mustJSON(t, map[string]any{
		"command": "echo hello-ship",
	}))
	if !strings.Contains(out, "hello-ship") {
		t.Fatalf("bash echo: %q", out)
	}
}

func TestConfirmWriteCancelled(t *testing.T) {
	dir := t.TempDir()
	sb, _ := NewSandbox(dir)
	env := &ToolEnv{
		Sandbox: sb,
		Yes:     false,
		ConfirmWrite: func(path string) (bool, error) {
			return false, nil
		},
	}
	out := ExecTool(context.Background(), env, "write", mustJSON(t, map[string]any{
		"path":    "ship.toml",
		"content": "schema = 2\n",
	}))
	if !strings.Contains(out, "cancelled") {
		t.Fatalf("got %q", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "ship.toml")); !os.IsNotExist(err) {
		t.Fatal("file should not exist")
	}
}

func TestEditNonUniqueFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x x x"), 0o644); err != nil {
		t.Fatal(err)
	}
	sb, _ := NewSandbox(dir)
	env := &ToolEnv{Sandbox: sb, Yes: true}
	out := ExecTool(context.Background(), env, "edit", mustJSON(t, map[string]any{
		"path":       "a.txt",
		"old_string": "x",
		"new_string": "y",
	}))
	if !strings.Contains(out, "error:") || !strings.Contains(out, "times") {
		t.Fatalf("got %q", out)
	}
}

func TestMaxTurns(t *testing.T) {
	dir := t.TempDir()
	sb, _ := NewSandbox(dir)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"role": "assistant",
						"tool_calls": []any{
							map[string]any{
								"id":   "c1",
								"type": "function",
								"function": map[string]any{
									"name":      "bash",
									"arguments": `{"command":"echo loop"}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
		})
	}))
	defer srv.Close()

	p, err := NewProvider("k", srv.URL+"/v1", "m")
	if err != nil {
		t.Fatal(err)
	}
	p.HTTPClient = srv.Client()
	agent, err := NewAgent(Config{Provider: p, Sandbox: sb, MaxTurns: 2, Yes: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = agent.RunUser(context.Background(), "loop forever")
	if err == nil || !strings.Contains(err.Error(), "max turns") {
		t.Fatalf("expected max turns error, got %v", err)
	}
}

func TestMultiToolCallInOneTurn(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	sb, _ := NewSandbox(dir)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{
					map[string]any{
						"message": map[string]any{
							"role": "assistant",
							"tool_calls": []any{
								map[string]any{
									"id":   "1",
									"type": "function",
									"function": map[string]any{
										"name":      "read",
										"arguments": `{"path":"a.txt"}`,
									},
								},
								map[string]any{
									"id":   "2",
									"type": "function",
									"function": map[string]any{
										"name":      "find",
										"arguments": `{"glob":"*.txt"}`,
									},
								},
							},
						},
						"finish_reason": "tool_calls",
					},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"role":    "assistant",
						"content": "saw both",
					},
					"finish_reason": "stop",
				},
			},
		})
	}))
	defer srv.Close()

	p, _ := NewProvider("k", srv.URL+"/v1", "m")
	p.HTTPClient = srv.Client()
	agent, _ := NewAgent(Config{Provider: p, Sandbox: sb, MaxTurns: 5, Yes: true})
	text, err := agent.RunUser(context.Background(), "read and find")
	if err != nil {
		t.Fatal(err)
	}
	if text != "saw both" {
		t.Fatalf("got %q", text)
	}
	toolN := 0
	for _, m := range agent.Messages() {
		if m.Role == "tool" {
			toolN++
		}
	}
	if toolN != 2 {
		t.Fatalf("expected 2 tool msgs, got %d", toolN)
	}
}

func TestAgentWriteInvalidThenFix(t *testing.T) {
	dir := t.TempDir()
	sb, _ := NewSandbox(dir)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		switch calls {
		case 1:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{
					"message": map[string]any{
						"role": "assistant",
						"tool_calls": []any{map[string]any{
							"id":   "w1",
							"type": "function",
							"function": map[string]any{
								"name":      "write",
								"arguments": `{"path":"ship.toml","content":"schema = 1\n"}`,
							},
						}},
					},
					"finish_reason": "tool_calls",
				}},
			})
		case 2:
			// Model "reads" the validation error from previous tool result and fixes.
			body := `schema = 2
[project]
name = "demo"
[version]
source = "static"
static = "0.0.1"
fallback = "static"
[features]
publish = false
deploy = false
rollback = false
verify = false
[build]
driver = "docker"
[build.docker]
image = "demo"
dockerfile = "./Dockerfile"
platforms = ["linux/amd64"]
load = true
[publish]
driver = "none"
[deploy]
driver = "none"
`
			args, _ := json.Marshal(map[string]string{"path": "ship.toml", "content": body})
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{
					"message": map[string]any{
						"role": "assistant",
						"tool_calls": []any{map[string]any{
							"id":   "w2",
							"type": "function",
							"function": map[string]any{
								"name":      "write",
								"arguments": string(args),
							},
						}},
					},
					"finish_reason": "tool_calls",
				}},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{
					"message":       map[string]any{"role": "assistant", "content": "fixed"},
					"finish_reason": "stop",
				}},
			})
		}
	}))
	defer srv.Close()

	p, _ := NewProvider("k", srv.URL+"/v1", "m")
	p.HTTPClient = srv.Client()
	agent, _ := NewAgent(Config{Provider: p, Sandbox: sb, MaxTurns: 5, Yes: true})
	text, err := agent.RunUser(context.Background(), "write ship.toml")
	if err != nil {
		t.Fatal(err)
	}
	if text != "fixed" {
		t.Fatalf("got %q", text)
	}
	var sawBad, sawOK bool
	for _, m := range agent.Messages() {
		if m.Role != "tool" {
			continue
		}
		if strings.Contains(m.Content, "validation:") && strings.Contains(m.Content, "schema") {
			sawBad = true
		}
		if strings.Contains(m.Content, "validation: ok") {
			sawOK = true
		}
	}
	if !sawBad || !sawOK {
		t.Fatalf("expected bad then ok validation in tool results (bad=%v ok=%v)", sawBad, sawOK)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
