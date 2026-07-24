package cmd

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/heyoungai/ship/internal"
	"github.com/pterm/pterm"
	"golang.org/x/term"
)

// aiSessionReporter is a line-mode REPL reporter: spinner until first token,
// streaming assistant text, and styled tool lines. Non-TTY skips spinner/color chrome.
type aiSessionReporter struct {
	out io.Writer
	tty bool

	mu        sync.Mutex
	spinner   *pterm.SpinnerPrinter
	streaming bool
	wrote     bool
}

func newAISessionReporter(out io.Writer) *aiSessionReporter {
	if out == nil {
		out = os.Stdout
	}
	tty := false
	if f, ok := out.(*os.File); ok {
		tty = term.IsTerminal(int(f.Fd()))
	}
	return &aiSessionReporter{out: out, tty: tty}
}

func (r *aiSessionReporter) OnTurnStart() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.streaming = false
	r.wrote = false
	r.stopSpinnerLocked()
	if !r.tty {
		return
	}
	sp, _ := pterm.DefaultSpinner.
		WithRemoveWhenDone(true).
		WithText("thinking…").
		Start()
	r.spinner = sp
}

func (r *aiSessionReporter) OnAssistantDelta(text string) {
	if text == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopSpinnerLocked()
	if !r.streaming {
		r.streaming = true
		if r.tty {
			fmt.Fprint(r.out, internal.DimStyle.Render("advisor")+" ")
		}
	}
	fmt.Fprint(r.out, text)
	r.wrote = true
}

func (r *aiSessionReporter) OnAssistantEnd() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopSpinnerLocked()
	if r.wrote {
		fmt.Fprintln(r.out)
		r.wrote = false
	}
	r.streaming = false
}

func (r *aiSessionReporter) OnToolStart(name, summary string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopSpinnerLocked()
	if r.wrote {
		fmt.Fprintln(r.out)
		r.wrote = false
	}
	r.streaming = false
	line := "→ " + name
	if summary != "" {
		line += " " + summary
	}
	if r.tty {
		fmt.Fprintln(r.out, internal.DimStyle.Render(line))
	} else {
		fmt.Fprintln(r.out, line)
	}
}

func (r *aiSessionReporter) OnToolEnd(name string, ok bool, detail string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	mark := "✓"
	style := internal.SuccessStyle
	if !ok {
		mark = "✗"
		style = internal.ErrorStyle
	}
	line := mark + " " + name
	if detail != "" {
		line += " — " + detail
	}
	if r.tty {
		fmt.Fprintln(r.out, style.Render(line))
	} else {
		fmt.Fprintln(r.out, line)
	}
}

func (r *aiSessionReporter) stopSpinnerLocked() {
	if r.spinner != nil {
		_ = r.spinner.Stop()
		r.spinner = nil
	}
}
