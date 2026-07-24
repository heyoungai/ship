package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatStreamSSEContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
			http.Error(w, err.Error(), 500)
			return
		}
		if !req.Stream {
			t.Error("expected stream=true")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		writeSSE(w, flusher, `{"choices":[{"delta":{"content":"Hel"}}]}`)
		writeSSE(w, flusher, `{"choices":[{"delta":{"content":"lo"}}]}`)
		writeSSE(w, flusher, `{"choices":[{"delta":{},"finish_reason":"stop"}]}`)
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer srv.Close()

	p, err := NewProvider("k", srv.URL+"/v1", "m")
	if err != nil {
		t.Fatal(err)
	}
	p.HTTPClient = srv.Client()

	var got strings.Builder
	msg, err := p.ChatStream(context.Background(), []Message{{Role: "user", Content: "hi"}}, nil, func(d string) {
		got.WriteString(d)
	})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "Hello" || got.String() != "Hello" {
		t.Fatalf("content=%q deltas=%q", msg.Content, got.String())
	}
	if len(msg.ToolCalls) != 0 {
		t.Fatalf("unexpected tool calls: %+v", msg.ToolCalls)
	}
}

func TestChatStreamSSEToolCallsAssembled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		// Name only on first chunk; arguments split across chunks.
		writeSSE(w, flusher, `{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"read","arguments":"{\"pa"}}]}}]}`)
		writeSSE(w, flusher, `{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"th\":\"a.txt\"}"}}]}}]}`)
		writeSSE(w, flusher, `{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`)
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	p, _ := NewProvider("k", srv.URL+"/v1", "m")
	p.HTTPClient = srv.Client()
	msg, err := p.ChatStream(context.Background(), []Message{{Role: "user", Content: "r"}}, ToolDefs(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("tool calls: %+v", msg.ToolCalls)
	}
	tc := msg.ToolCalls[0]
	if tc.ID != "call_1" || tc.Function.Name != "read" {
		t.Fatalf("got %+v", tc)
	}
	if tc.Function.Arguments != `{"path":"a.txt"}` {
		t.Fatalf("args %q", tc.Function.Arguments)
	}
}

func TestChatStreamJSONFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{
				map[string]any{
					"message":       map[string]any{"role": "assistant", "content": "plain"},
					"finish_reason": "stop",
				},
			},
		})
	}))
	defer srv.Close()

	p, _ := NewProvider("k", srv.URL+"/v1", "m")
	p.HTTPClient = srv.Client()
	var deltas string
	msg, err := p.ChatStream(context.Background(), []Message{{Role: "user", Content: "x"}}, nil, func(d string) {
		deltas += d
	})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "plain" || deltas != "plain" {
		t.Fatalf("content=%q deltas=%q", msg.Content, deltas)
	}
}

func TestChatStreamAPIErrorFrame(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("data: {\"error\":{\"message\":\"boom\",\"type\":\"invalid\"}}\n\n"))
	}))
	defer srv.Close()

	p, _ := NewProvider("k", srv.URL+"/v1", "m")
	p.HTTPClient = srv.Client()
	_, err := p.ChatStream(context.Background(), []Message{{Role: "user", Content: "x"}}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("got %v", err)
	}
}

func TestToolCallSummaryAndResultOK(t *testing.T) {
	if s := ToolCallSummary("read", `{"path":"ship.toml"}`); s != "ship.toml" {
		t.Fatalf("summary %q", s)
	}
	if ToolResultOK("error: nope") {
		t.Fatal("expected not ok")
	}
	if !ToolResultOK("edited ship.toml (1 replacement(s))") {
		t.Fatal("expected ok")
	}
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, payload string) {
	_, _ = w.Write([]byte("data: " + payload + "\n\n"))
	if flusher != nil {
		flusher.Flush()
	}
}
