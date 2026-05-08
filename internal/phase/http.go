package phase

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"

	"subwofer/internal/config"
	"subwofer/internal/normalize"
	"subwofer/internal/runner"
	"subwofer/internal/tui"
)

// RunHTTP runs httpx against resolved (or all) subdomains.
func RunHTTP(ctx context.Context, cfg *config.Config, r *tui.Renderer) error {
	r.PrintPhaseHeader("Phase 5 — HTTP Probing")

	if !runner.HasCmd("httpx") {
		r.PrintLine("  \033[33m~\033[0m httpx not installed — skipping")
		return nil
	}

	// Prefer the resolved list; fall back to all subdomains.
	resolvedPath := filepath.Join(cfg.OutDir, "subdomains_resolved.txt")
	subsPath := filepath.Join(cfg.OutDir, "subdomains.txt")
	liveURLs := filepath.Join(cfg.OutDir, "live_urls.txt")

	inputPath := subsPath
	if normalize.CountLines(resolvedPath) > 0 {
		inputPath = resolvedPath
	}

	state := tui.NewPhaseState([]string{"httpx"})
	done := make(chan struct{})
	r.Start(state, done)

	state.Update("httpx", func(ts *tui.ToolState) { ts.Status = tui.StatusRunning })
	if !r.IsTerminal() {
		r.PrintLine("[+] httpx: running")
	}

	cmd := exec.CommandContext(ctx, "httpx",
		"-l", inputPath,
		"-silent",
		"-t", strconv.Itoa(cfg.Threads),
		"-status-code",
		"-title",
		"-tech-detect",
		"-o", liveURLs,
	)

	stderrW, cleanupStderr := verboseWriter(cfg.Verbose, state, "httpx")
	cmd.Stderr = stderrW

	err := cmd.Run()
	cleanupStderr()

	count := normalize.CountLines(liveURLs)
	state.Update("httpx", func(ts *tui.ToolState) {
		if err != nil && ctx.Err() == nil {
			ts.Status = tui.StatusFailed
			ts.Message = err.Error()
		} else {
			ts.Status = tui.StatusDone
			ts.Count = count
		}
	})
	if !r.IsTerminal() {
		r.PrintLine("[✓] httpx: %d results", count)
	}

	close(done)
	return nil
}
