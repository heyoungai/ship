package ai

import (
	"context"
	"fmt"
	"io"
	"os"
)

// Config configures a single agent run (one user turn through tool loop).
type Config struct {
	Provider     *Provider
	Sandbox      *Sandbox
	System       string
	MaxTurns     int
	DryRun       bool
	Yes          bool
	ConfirmWrite func(path string) (bool, error)
	// Writer receives assistant text when Reporter is nil (legacy / tests).
	Writer io.Writer
	// Reporter receives streaming deltas and tool events. If nil, a WriterReporter
	// is created from Writer (ShowTools follows ToolTrace).
	Reporter Reporter
	// ToolTrace when true shows tool lines on the default WriterReporter.
	// CLI typically sets an explicit Reporter that always shows tools.
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
	if cfg.Reporter == nil {
		cfg.Reporter = &WriterReporter{Out: cfg.Writer, ShowTools: cfg.ToolTrace}
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
	rep := a.cfg.Reporter
	if rep == nil {
		rep = NopReporter{}
	}
	var lastText string
	for turn := 0; turn < a.cfg.MaxTurns; turn++ {
		rep.OnTurnStart()
		msg, err := a.cfg.Provider.ChatStream(ctx, a.messages, a.tools, rep.OnAssistantDelta)
		if err != nil {
			return lastText, err
		}
		a.messages = append(a.messages, msg)
		if len(msg.ToolCalls) == 0 {
			lastText = msg.Content
			rep.OnAssistantEnd()
			return lastText, nil
		}
		if msg.Content != "" {
			lastText = msg.Content
			rep.OnAssistantEnd()
		}
		for _, tc := range msg.ToolCalls {
			summary := ToolCallSummary(tc.Function.Name, tc.Function.Arguments)
			rep.OnToolStart(tc.Function.Name, summary)
			result := ExecTool(ctx, a.env, tc.Function.Name, tc.Function.Arguments)
			ok := ToolResultOK(result)
			rep.OnToolEnd(tc.Function.Name, ok, ToolResultDetail(result))
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
