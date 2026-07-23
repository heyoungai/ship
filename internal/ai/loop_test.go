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

func TestSandboxRejectsEscape(t *testing.T) {
	dir := t.TempDir()
	sb, err := NewSandbox(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sb.Resolve("../outside"); err == nil {
		t.Fatal("expected escape to fail")
	}
	ok, err := sb.Resolve("ship.toml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ok, sb.Root) {
		t.Fatalf("resolved path %q not under root %q", ok, sb.Root)
	}
}

func TestGuardBashBlocksDeploy(t *testing.T) {
	cases := []string{
		"ship deploy",
		"ship run -y",
		"./ship push",
		"ship.exe rollback",
		"echo hi && ship deploy",
	}
	for _, c := range cases {
		if err := GuardBash(c); err == nil {
			t.Fatalf("expected block for %q", c)
		}
	}
	if err := GuardBash("ship plan --json"); err != nil {
		t.Fatalf("plan should be allowed: %v", err)
	}
	if err := GuardBash("ship doctor"); err != nil {
		t.Fatalf("doctor should be allowed: %v", err)
	}
}

func TestToolWriteDryRunAndSandbox(t *testing.T) {
	dir := t.TempDir()
	sb, err := NewSandbox(dir)
	if err != nil {
		t.Fatal(err)
	}
	env := &ToolEnv{Sandbox: sb, DryRun: true}
	out := ExecTool(context.Background(), env, "write", `{"path":"ship.toml","content":"schema = 2\n"}`)
	if !strings.Contains(out, "dry-run") {
		t.Fatalf("expected dry-run output, got %q", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "ship.toml")); !os.IsNotExist(err) {
		t.Fatal("dry-run should not create file")
	}

	env.DryRun = false
	env.Yes = true
	out = ExecTool(context.Background(), env, "write", `{"path":"../x.toml","content":"no"}`)
	if !strings.Contains(out, "outside project root") {
		t.Fatalf("expected sandbox error, got %q", out)
	}
}

func TestToolBashBlocked(t *testing.T) {
	dir := t.TempDir()
	sb, _ := NewSandbox(dir)
	env := &ToolEnv{Sandbox: sb}
	out := ExecTool(context.Background(), env, "bash", `{"command":"ship deploy"}`)
	if !strings.Contains(out, "blocked") {
		t.Fatalf("expected blocked, got %q", out)
	}
}

func TestAgentLoopWithMockHTTP(t *testing.T) {
	dir := t.TempDir()
	sb, err := NewSandbox(dir)
	if err != nil {
		t.Fatal(err)
	}

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			// ask to find files
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{
					map[string]any{
						"message": map[string]any{
							"role": "assistant",
							"tool_calls": []any{
								map[string]any{
									"id":   "call_1",
									"type": "function",
									"function": map[string]any{
										"name":      "find",
										"arguments": `{"path":".","glob":"*"}`,
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
						"content": "done exploring",
					},
					"finish_reason": "stop",
				},
			},
		})
	}))
	defer srv.Close()

	provider, err := NewProvider("test-key", srv.URL+"/v1", "test-model")
	if err != nil {
		t.Fatal(err)
	}
	provider.HTTPClient = srv.Client()

	agent, err := NewAgent(Config{
		Provider: provider,
		Sandbox:  sb,
		MaxTurns: 5,
		Yes:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	text, err := agent.RunUser(context.Background(), "look around")
	if err != nil {
		t.Fatal(err)
	}
	if text != "done exploring" {
		t.Fatalf("got %q", text)
	}
	if calls != 2 {
		t.Fatalf("expected 2 API calls, got %d", calls)
	}
	// tool result should be in history
	foundTool := false
	for _, m := range agent.Messages() {
		if m.Role == "tool" && m.Name == "find" {
			foundTool = true
		}
	}
	if !foundTool {
		t.Fatal("expected find tool message in history")
	}
}

func TestEditAndRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	sb, _ := NewSandbox(dir)
	env := &ToolEnv{Sandbox: sb, Yes: true}
	out := ExecTool(context.Background(), env, "edit", `{"path":"a.txt","old_string":"world","new_string":"ship"}`)
	if strings.Contains(out, "error") {
		t.Fatal(out)
	}
	got := ExecTool(context.Background(), env, "read", `{"path":"a.txt"}`)
	if got != "hello ship" {
		t.Fatalf("got %q", got)
	}
}
