package ai

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Reporter receives streaming / tool events during an agent turn.
type Reporter interface {
	OnTurnStart()
	OnAssistantDelta(text string)
	OnAssistantEnd()
	OnToolStart(name, summary string)
	OnToolEnd(name string, ok bool, detail string)
}

// NopReporter discards all events.
type NopReporter struct{}

func (NopReporter) OnTurnStart()                              {}
func (NopReporter) OnAssistantDelta(string)                   {}
func (NopReporter) OnAssistantEnd()                           {}
func (NopReporter) OnToolStart(string, string)                {}
func (NopReporter) OnToolEnd(string, bool, string)            {}

// WriterReporter streams assistant text to Writer and optionally prints tool lines.
type WriterReporter struct {
	Out       io.Writer
	ShowTools bool
	wrote     bool
}

func (r *WriterReporter) OnTurnStart() {}

func (r *WriterReporter) OnAssistantDelta(text string) {
	if r == nil || r.Out == nil || text == "" {
		return
	}
	fmt.Fprint(r.Out, text)
	r.wrote = true
}

func (r *WriterReporter) OnAssistantEnd() {
	if r == nil || r.Out == nil {
		return
	}
	if r.wrote {
		fmt.Fprintln(r.Out)
		r.wrote = false
	}
}

func (r *WriterReporter) OnToolStart(name, summary string) {
	if r == nil || r.Out == nil || !r.ShowTools {
		return
	}
	if r.wrote {
		fmt.Fprintln(r.Out)
		r.wrote = false
	}
	line := "→ " + name
	if summary != "" {
		line += " " + summary
	}
	fmt.Fprintln(r.Out, line)
}

func (r *WriterReporter) OnToolEnd(name string, ok bool, detail string) {
	if r == nil || r.Out == nil || !r.ShowTools {
		return
	}
	mark := "✓"
	if !ok {
		mark = "✗"
	}
	line := mark + " " + name
	if detail != "" {
		line += " — " + detail
	}
	fmt.Fprintln(r.Out, line)
}

// ToolCallSummary extracts a short human-readable hint from tool arguments JSON.
func ToolCallSummary(name, argsJSON string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &m); err != nil {
		return ""
	}
	switch name {
	case "read", "write", "edit":
		return strField(m, "path")
	case "bash":
		return truncate(strField(m, "command"), 72)
	case "grep":
		pat := strField(m, "pattern")
		path := strField(m, "path")
		if path != "" {
			return truncate(pat, 40) + " in " + path
		}
		return truncate(pat, 56)
	case "find":
		g := strField(m, "glob")
		path := strField(m, "path")
		if path != "" && path != "." {
			return g + " under " + path
		}
		return g
	default:
		return ""
	}
}

func strField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// ToolResultOK reports whether a tool result looks successful.
func ToolResultOK(result string) bool {
	s := strings.TrimSpace(result)
	if s == "" {
		return true
	}
	if strings.HasPrefix(s, "error:") || strings.HasPrefix(s, "error ") {
		return false
	}
	if strings.Contains(s, "\nerror:") {
		return false
	}
	return true
}

// ToolResultDetail is a one-line detail for tool end events.
func ToolResultDetail(result string) string {
	s := strings.TrimSpace(result)
	if s == "" {
		return ""
	}
	line := s
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		line = s[:i]
	}
	return truncate(line, 80)
}
