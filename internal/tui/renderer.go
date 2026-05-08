package tui

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/term"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Renderer handles all TUI output to os.Stderr.
type Renderer struct {
	verbose    bool
	isTTY      bool
	drawnLines int // number of lines drawn in the last render pass
}

// NewRenderer creates a Renderer, auto-detecting TTY.
func NewRenderer(verbose bool) *Renderer {
	return &Renderer{
		verbose: verbose,
		isTTY:   term.IsTerminal(int(os.Stderr.Fd())),
	}
}

// IsTerminal returns true when stderr is a real TTY.
func (r *Renderer) IsTerminal() bool { return r.isTTY }

// HideCursor sends the ANSI escape to hide the cursor.
func (r *Renderer) HideCursor() {
	if r.isTTY {
		fmt.Fprint(os.Stderr, "\033[?25l")
	}
}

// ShowCursor restores the cursor.
func (r *Renderer) ShowCursor() {
	if r.isTTY {
		fmt.Fprint(os.Stderr, "\033[?25h")
	}
}

// PrintPhaseHeader prints a coloured phase heading.
func (r *Renderer) PrintPhaseHeader(name string) {
	fmt.Fprintf(os.Stderr, "\n%s%s[*] %s%s\n", colorCyan, colorBold, name, colorReset)
}

// PrintLine prints a formatted line when no render loop is active.
func (r *Renderer) PrintLine(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// Start launches a background goroutine that redraws the tool table every
// 80 ms. It returns once done is closed (final redraw already performed).
func (r *Renderer) Start(state *PhaseState, done <-chan struct{}) {
	if !r.isTTY {
		// Non-TTY: just wait for completion; individual updates printed elsewhere.
		go func() {
			<-done
		}()
		return
	}

	r.HideCursor()

	go func() {
		defer r.ShowCursor()

		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()

		frame := 0

		for {
			select {
			case <-done:
				r.draw(state, frame)
				fmt.Fprintln(os.Stderr)
				return
			case <-ticker.C:
				r.draw(state, frame)
				frame = (frame + 1) % len(spinnerFrames)
			}
		}
	}()
}

// draw redraws the tool table in place.
func (r *Renderer) draw(state *PhaseState, frame int) {
	tools, logs := state.Snapshot()

	// Move cursor up to overwrite previously drawn lines.
	if r.drawnLines > 0 {
		fmt.Fprintf(os.Stderr, "\033[%dA", r.drawnLines)
	}

	lines := 0

	for _, t := range tools {
		line := r.formatToolLine(t, frame)
		fmt.Fprintf(os.Stderr, "\033[2K\r%s\n", line)
		lines++
	}

	if r.verbose && len(logs) > 0 {
		// Separator.
		fmt.Fprintf(os.Stderr, "\033[2K\r%s────────────────────────────────────────%s\n", colorDim, colorReset)
		lines++

		// Show last 8 log lines.
		start := 0
		if len(logs) > 8 {
			start = len(logs) - 8
		}
		for _, l := range logs[start:] {
			display := l
			if len(display) > 100 {
				display = display[:100] + "…"
			}
			fmt.Fprintf(os.Stderr, "\033[2K\r%s%s%s\n", colorDim, display, colorReset)
			lines++
		}
	}

	r.drawnLines = lines
}

// formatToolLine renders a single tool row.
func (r *Renderer) formatToolLine(t ToolState, frame int) string {
	name := fmt.Sprintf("%-24s", t.Name)

	switch t.Status {
	case StatusPending:
		return fmt.Sprintf("  %s%s%s  %s·%s pending", colorDim, name, colorReset, colorDim, colorReset)
	case StatusRunning:
		spinner := spinnerFrames[frame]
		return fmt.Sprintf("  %s%s%s  %s%s%s running...", colorCyan, name, colorReset, colorCyan, spinner, colorReset)
	case StatusDone:
		return fmt.Sprintf("  %s%s%s  %s✓%s %d results", colorBold, name, colorReset, colorGreen, colorReset, t.Count)
	case StatusSkipped:
		msg := t.Message
		if msg == "" {
			msg = "skipped"
		}
		return fmt.Sprintf("  %s%s%s  %s~%s %s", colorDim, name, colorReset, colorYellow, colorReset, msg)
	case StatusFailed:
		msg := t.Message
		if msg == "" {
			msg = "failed"
		}
		return fmt.Sprintf("  %s%s%s  %s✗%s %s", colorBold, name, colorReset, colorRed, colorReset, msg)
	default:
		return fmt.Sprintf("  %s  ?", name)
	}
}

// PrintNonTTYUpdate prints a simple status line for non-TTY environments.
func (r *Renderer) PrintNonTTYUpdate(t ToolState) {
	if r.isTTY {
		return
	}
	switch t.Status {
	case StatusRunning:
		fmt.Fprintf(os.Stderr, "[+] %s: running\n", t.Name)
	case StatusDone:
		fmt.Fprintf(os.Stderr, "[✓] %s: %d results\n", t.Name, t.Count)
	case StatusSkipped:
		fmt.Fprintf(os.Stderr, "[~] %s: %s\n", t.Name, t.Message)
	case StatusFailed:
		fmt.Fprintf(os.Stderr, "[✗] %s: %s\n", t.Name, t.Message)
	}
}

// PrintSummary prints the final results summary box to stderr.
func (r *Renderer) PrintSummary(files map[string]int, paths map[string]string) {
	const width = 50
	border := fmt.Sprintf(" ╔%s╗", repeatRune('═', width))
	title := fmt.Sprintf(" ║%*s%-*s║", (width+len("SUBWOFER — Results Summary"))/2, "SUBWOFER — Results Summary", width-(width+len("SUBWOFER — Results Summary"))/2, "")
	mid := fmt.Sprintf(" ╠%s╣", repeatRune('═', width))
	bottom := fmt.Sprintf(" ╚%s╝", repeatRune('═', width))

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, border)
	fmt.Fprintln(os.Stderr, title)
	fmt.Fprintln(os.Stderr, mid)

	order := []string{"all subdomains", "resolved", "live URLs", "screenshots"}
	for _, key := range order {
		count, ok := files[key]
		if !ok {
			continue
		}
		path := paths[key]
		countStr := fmt.Sprintf("%5d", count)
		row := fmt.Sprintf(" ║  %-16s:  %s  →  %s", key, countStr, path)
		fmt.Fprintln(os.Stderr, row)
	}

	fmt.Fprintln(os.Stderr, bottom)
	fmt.Fprintln(os.Stderr)
}

func repeatRune(r rune, n int) string {
	out := make([]rune, n)
	for i := range out {
		out[i] = r
	}
	return string(out)
}
