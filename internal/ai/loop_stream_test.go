package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type recordingReporter struct {
	deltas []string
	tools  []string
	ends   []string
}

func (r *recordingReporter) OnTurnStart() {}
func (r *recordingReporter) OnAssistantDelta(text string) {
	r.deltas = append(r.deltas, text)
}
func (r *recordingReporter) OnAssistantEnd() {}
func (r *recordingReporter) OnToolStart(name, summary string) {
	r.tools = append(r.tools, name+":"+summary)
}
func (r *recordingReporter) OnToolEnd(name string, ok bool, detail string) {
	mark := "ok"
	if !ok {
		mark = "err"
	}
	r.ends = append(r.ends, name+":"+mark)
}

func TestAgentLoopStreamingReporter(t *testing.T) {
	dir := t.TempDir()
	sb, err := NewSandbox(dir)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		if calls == 1 {
			writeSSE(w, flusher, `{"choices":[{"delta":{"content":"looking…"}}]}`)
			writeSSE(w, flusher, `{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"find","arguments":"{\"glob\":\"*\"}"}}]}}]}`)
			writeSSE(w, flusher, `{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`)
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			return
		}
		writeSSE(w, flusher, `{"choices":[{"delta":{"content":"all"}}]}`)
		writeSSE(w, flusher, `{"choices":[{"delta":{"content":" good"}}]}`)
		writeSSE(w, flusher, `{"choices":[{"delta":{},"finish_reason":"stop"}]}`)
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	p, err := NewProvider("k", srv.URL+"/v1", "m")
	if err != nil {
		t.Fatal(err)
	}
	p.HTTPClient = srv.Client()
	rep := &recordingReporter{}
	agent, err := NewAgent(Config{
		Provider: p,
		Sandbox:  sb,
		MaxTurns: 5,
		Yes:      true,
		Reporter: rep,
	})
	if err != nil {
		t.Fatal(err)
	}
	text, err := agent.RunUser(context.Background(), "explore")
	if err != nil {
		t.Fatal(err)
	}
	if text != "all good" {
		t.Fatalf("got %q", text)
	}
	joined := strings.Join(rep.deltas, "")
	if !strings.Contains(joined, "looking") || !strings.Contains(joined, "all good") {
		t.Fatalf("deltas %v", rep.deltas)
	}
	if len(rep.tools) != 1 || !strings.HasPrefix(rep.tools[0], "find:") {
		t.Fatalf("tools %v", rep.tools)
	}
	if len(rep.ends) != 1 || rep.ends[0] != "find:ok" {
		t.Fatalf("ends %v", rep.ends)
	}
}

func TestWriterReporterShowTools(t *testing.T) {
	var buf strings.Builder
	r := &WriterReporter{Out: &buf, ShowTools: true}
	r.OnAssistantDelta("hi")
	r.OnAssistantEnd()
	r.OnToolStart("read", "a.txt")
	r.OnToolEnd("read", true, "ok")
	out := buf.String()
	if !strings.Contains(out, "hi\n") || !strings.Contains(out, "→ read a.txt") || !strings.Contains(out, "✓ read") {
		t.Fatalf("got %q", out)
	}
}

// Ensure JSON mock still works via ChatStream fallback (used by older tests).
func TestAgentLoopJSONFallbackStillWorks(t *testing.T) {
	dir := t.TempDir()
	sb, _ := NewSandbox(dir)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message":       map[string]any{"role": "assistant", "content": "ok"},
				"finish_reason": "stop",
			}},
		})
	}))
	defer srv.Close()
	p, _ := NewProvider("k", srv.URL+"/v1", "m")
	p.HTTPClient = srv.Client()
	agent, _ := NewAgent(Config{Provider: p, Sandbox: sb, MaxTurns: 2})
	text, err := agent.RunUser(context.Background(), "hi")
	if err != nil || text != "ok" {
		t.Fatalf("text=%q err=%v", text, err)
	}
}
