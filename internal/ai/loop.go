package ai

import (
	"context"
	"fmt"
	"io"
	"os"
)

// Config configures a single agent run (one user turn through tool loop).
type Config struct {
	Provider   *Provider
	Sandbox    *Sandbox
	System     string
	MaxTurns   int
	DryRun     bool
	Yes        bool
	ConfirmWrite func(path string) (bool, error)
	// Writer receives assistant text and tool traces (optional).
	Writer io.Writer
	// ToolTrace when true prints tool names to Writer.
	ToolTrace bool
}

// Agent runs a thin tool loop.
type Agent struct {
	cfg      Config
	messages []Message
	tools    []ToolDef
	env      *ToolEnv
}

func NewAgent(cfg Config) (*Agent, error) {
	if cfg.Provider == nil {
		return nil, fmt.Errorf("provider is required")
	}
	if cfg.Sandbox == nil {
		return nil, fmt.Errorf("sandbox is required")
	}
	if cfg.MaxTurns <= 0 {
		cfg.MaxTurns = 20
	}
	if cfg.System == "" {
		cfg.System = DefaultSystemPrompt
	}
	if cfg.Writer == nil {
		cfg.Writer = io.Discard
	}
	a := &Agent{
		cfg:   cfg,
		tools: ToolDefs(),
		env: &ToolEnv{
			Sandbox:      cfg.Sandbox,
			DryRun:       cfg.DryRun,
			Yes:          cfg.Yes,
			ConfirmWrite: cfg.ConfirmWrite,
		},
		messages: []Message{
			{Role: "system", Content: cfg.System},
		},
	}
	return a, nil
}

// Messages returns a copy of the conversation (without mutating).
func (a *Agent) Messages() []Message {
	out := make([]Message, len(a.messages))
	copy(out, a.messages)
	return out
}

// RunUser appends a user message and runs the tool loop until the model stops calling tools or MaxTurns.
func (a *Agent) RunUser(ctx context.Context, userText string) (string, error) {
	a.messages = append(a.messages, Message{Role: "user", Content: userText})
	var lastText string
	for turn := 0; turn < a.cfg.MaxTurns; turn++ {
		msg, err := a.cfg.Provider.Chat(ctx, a.messages, a.tools)
		if err != nil {
			return lastText, err
		}
		a.messages = append(a.messages, msg)
		if len(msg.ToolCalls) == 0 {
			lastText = msg.Content
			if msg.Content != "" {
				fmt.Fprintln(a.cfg.Writer, msg.Content)
			}
			return lastText, nil
		}
		if msg.Content != "" {
			fmt.Fprintln(a.cfg.Writer, msg.Content)
			lastText = msg.Content
		}
		for _, tc := range msg.ToolCalls {
			if a.cfg.ToolTrace {
				fmt.Fprintf(a.cfg.Writer, "[tool] %s\n", tc.Function.Name)
			}
			result := ExecTool(ctx, a.env, tc.Function.Name, tc.Function.Arguments)
			a.messages = append(a.messages, Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
				Content:    result,
			})
		}
	}
	return lastText, fmt.Errorf("reached max turns (%d)", a.cfg.MaxTurns)
}

// ResolveProviderFromEnv builds a provider from environment and overrides.
func ResolveProviderFromEnv(model, baseURL string) (*Provider, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if baseURL == "" {
		baseURL = os.Getenv("OPENAI_BASE_URL")
	}
	if model == "" {
		model = os.Getenv("SHIP_AI_MODEL")
	}
	return NewProvider(apiKey, baseURL, model)
}
