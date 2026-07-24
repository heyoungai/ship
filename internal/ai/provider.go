package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Message is a chat message in OpenAI-compatible format.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolDef struct {
	Type     string          `json:"type"`
	Function ToolFunctionDef `json:"function"`
}

type ToolFunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type chatRequest struct {
	Model      string    `json:"model"`
	Messages   []Message `json:"messages"`
	Tools      []ToolDef `json:"tools,omitempty"`
	ToolChoice any       `json:"tool_choice,omitempty"`
	Stream     bool      `json:"stream,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string            `json:"content"`
			ToolCalls []streamToolDelta `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

type streamToolDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Provider talks to an OpenAI-compatible Chat Completions API.
type Provider struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

func NewProvider(apiKey, baseURL, model string) (*Provider, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("missing API key: set OPENAI_API_KEY (or use a deterministic ship init)")
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = "gpt-4.1-mini"
	}
	return &Provider{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
		// No overall Timeout: streaming completions can exceed fixed deadlines; rely on ctx.
		HTTPClient: &http.Client{
			Timeout: 0,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				ResponseHeaderTimeout: 120 * time.Second,
			},
		},
	}, nil
}

// Chat sends a chat completion request (delegates to ChatStream with no delta callback).
func (p *Provider) Chat(ctx context.Context, messages []Message, tools []ToolDef) (Message, error) {
	return p.ChatStream(ctx, messages, tools, nil)
}

// ChatStream requests stream=true, invokes onDelta for each content delta, and returns the
// assembled assistant Message. If the server replies with non-SSE JSON (common in mocks /
// some gateways), it falls back to a single-shot parse and one onDelta call for content.
func (p *Provider) ChatStream(ctx context.Context, messages []Message, tools []ToolDef, onDelta func(string)) (Message, error) {
	reqBody := chatRequest{
		Model:    p.Model,
		Messages: messages,
		Tools:    tools,
		Stream:   true,
	}
	if len(tools) > 0 {
		reqBody.ToolChoice = "auto"
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return Message{}, err
	}
	url := p.BaseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return Message{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Accept", "text/event-stream")

	client := p.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return Message{}, err
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		// Try structured error first.
		var parsed chatResponse
		if json.Unmarshal(body, &parsed) == nil && parsed.Error != nil {
			return Message{}, fmt.Errorf("api error: %s", parsed.Error.Message)
		}
		return Message{}, fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(body), 500))
	}

	if isEventStream(ct) {
		return p.readSSE(resp.Body, onDelta)
	}

	// Peek: some servers ignore stream=true and return JSON.
	peek := make([]byte, 8)
	n, _ := io.ReadFull(resp.Body, peek)
	head := peek[:n]
	rest := io.MultiReader(bytes.NewReader(head), resp.Body)
	trimmed := bytes.TrimLeft(head, " \t\r\n")
	if len(trimmed) > 0 && trimmed[0] == '{' {
		return p.readJSONMessage(rest, onDelta)
	}
	if bytes.HasPrefix(trimmed, []byte("data:")) {
		return p.readSSE(rest, onDelta)
	}
	return p.readSSE(rest, onDelta)
}

func isEventStream(ct string) bool {
	return strings.Contains(strings.ToLower(ct), "text/event-stream")
}

func (p *Provider) readJSONMessage(r io.Reader, onDelta func(string)) (Message, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return Message{}, err
	}
	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Message{}, fmt.Errorf("decode response: %w", err)
	}
	if parsed.Error != nil {
		return Message{}, fmt.Errorf("api error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return Message{}, fmt.Errorf("empty choices in response")
	}
	msg := parsed.Choices[0].Message
	msg.Role = "assistant"
	if onDelta != nil && msg.Content != "" {
		onDelta(msg.Content)
	}
	return msg, nil
}

func (p *Provider) readSSE(r io.Reader, onDelta func(string)) (Message, error) {
	sc := bufio.NewScanner(r)
	// Tool-call argument streams can be large.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 4*1024*1024)

	var content strings.Builder
	builders := map[int]*toolCallBuilder{}
	maxIdx := -1

	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil {
			return Message{}, fmt.Errorf("api error: %s", chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta
		if delta.Content != "" {
			content.WriteString(delta.Content)
			if onDelta != nil {
				onDelta(delta.Content)
			}
		}
		for _, tc := range delta.ToolCalls {
			idx := tc.Index
			b, ok := builders[idx]
			if !ok {
				b = &toolCallBuilder{typ: "function"}
				builders[idx] = b
			}
			if idx > maxIdx {
				maxIdx = idx
			}
			if tc.ID != "" {
				b.id = tc.ID
			}
			if tc.Type != "" {
				b.typ = tc.Type
			}
			if tc.Function.Name != "" {
				b.name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				b.args += tc.Function.Arguments
			}
		}
	}
	if err := sc.Err(); err != nil {
		return Message{}, fmt.Errorf("read stream: %w", err)
	}

	msg := Message{
		Role:    "assistant",
		Content: content.String(),
	}
	if maxIdx >= 0 {
		msg.ToolCalls = make([]ToolCall, 0, maxIdx+1)
		for i := 0; i <= maxIdx; i++ {
			b, ok := builders[i]
			if !ok {
				continue
			}
			typ := b.typ
			if typ == "" {
				typ = "function"
			}
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{
				ID:   b.id,
				Type: typ,
				Function: ToolFunction{
					Name:      b.name,
					Arguments: b.args,
				},
			})
		}
	}
	return msg, nil
}

type toolCallBuilder struct {
	id, typ, name, args string
}

// DisplayHost returns a short host (or base URL) for banners; never includes credentials.
func (p *Provider) DisplayHost() string {
	if p == nil {
		return ""
	}
	u := strings.TrimPrefix(p.BaseURL, "https://")
	u = strings.TrimPrefix(u, "http://")
	if i := strings.IndexByte(u, '/'); i >= 0 {
		u = u[:i]
	}
	return u
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
