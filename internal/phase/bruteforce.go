package phase

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"subwofer/internal/config"
	"subwofer/internal/normalize"
	"subwofer/internal/output"
	"subwofer/internal/runner"
	"subwofer/internal/tui"
	"subwofer/internal/wordlist"
)

// RunBruteforce runs DNS brute-forcing using puredns (preferred) or shuffledns.
func RunBruteforce(ctx context.Context, cfg *config.Config, r *tui.Renderer) error {
	r.PrintPhaseHeader("Phase 2 — DNS Bruteforce")

	wl := cfg.Wordlist
	if wl == "" {
		builtinPath := filepath.Join(cfg.TmpDir, "builtin_wordlist.txt")
		n, err := wordlist.WriteToFile(builtinPath)
		if err != nil {
			r.PrintLine("  \033[33m~\033[0m No wordlist available — skipping")
			return nil
		}
		wl = builtinPath
		r.PrintLine("  \033[90m(using built-in wordlist: %d words)\033[0m", n)
	}

	var toolName string
	var buildCmd func(domain string) *exec.Cmd

	switch {
	case runner.HasCmd("puredns"):
		toolName = "puredns"
		buildCmd = func(d string) *exec.Cmd {
			return exec.CommandContext(ctx, "puredns", "bruteforce", wl, d, "--quiet")
		}
	case runner.HasCmd("shuffledns"):
		toolName = "shuffledns"
		buildCmd = func(d string) *exec.Cmd {
			return exec.CommandContext(ctx, "shuffledns", "-d", d, "-w", wl, "-silent")
		}
	default:
		r.PrintLine("  %s~%s puredns / shuffledns not installed — skipping", "\033[33m", "\033[0m")
		return nil
	}

	// Show a single spinner for brute force.
	spinnerState := tui.NewPhaseState([]string{toolName})
	spinDone := make(chan struct{})
	r.Start(spinnerState, spinDone)

	spinnerState.Update(toolName, func(ts *tui.ToolState) {
		ts.Status = tui.StatusRunning
	})
	if !r.IsTerminal() {
		r.PrintLine("[+] %s: running", toolName)
	}

	outPath := filepath.Join(cfg.SourcesDir, toolName+".txt")
	f, err := os.Create(outPath)
	if err != nil {
		close(spinDone)
		return fmt.Errorf("brute force output file: %w", err)
	}

	totalCount := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for _, d := range cfg.Domains {
		if ctx.Err() != nil {
			break
		}

		cmd := buildCmd(d)
		if cfg.Verbose {
			cmd.Stderr = os.Stderr
		} else {
			cmd.Stderr = nil
		}

		out, err := cmd.Output()
		if err != nil && ctx.Err() != nil {
			_ = f.Close()
			close(spinDone)
			return ctx.Err()
		}

		if len(out) > 0 {
			_, _ = f.Write(out)
			// Count newlines in this chunk.
			for _, b := range out {
				if b == '\n' {
					totalCount++
				}
			}
		}
	}
	_ = f.Close()

	rawPath := filepath.Join(cfg.TmpDir, "all_raw.txt")
	_ = output.AppendToFile(rawPath, outPath)

	spinnerState.Update(toolName, func(ts *tui.ToolState) {
		ts.Status = tui.StatusDone
		ts.Count = normalize.CountLines(outPath)
	})
	if !r.IsTerminal() {
		r.PrintLine("[✓] %s: %d results", toolName, normalize.CountLines(outPath))
	}

	close(spinDone)
	return nil
}
