package phase

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"

	"subwofer/internal/config"
	"subwofer/internal/normalize"
	"subwofer/internal/output"
	"subwofer/internal/runner"
	"subwofer/internal/tui"
)

// RunResolve resolves discovered subdomains using dnsx.
// If dnsx is unavailable the subdomains file is simply copied.
func RunResolve(ctx context.Context, cfg *config.Config, r *tui.Renderer) error {
	r.PrintPhaseHeader("Phase 4 — DNS Resolution")

	inputPath := filepath.Join(cfg.OutDir, "subdomains.txt")
	resolvedPath := filepath.Join(cfg.OutDir, "subdomains_resolved.txt")

	if !runner.HasCmd("dnsx") {
		r.PrintLine("  \033[33m~\033[0m dnsx not installed — copying subdomains.txt as resolved")
		return output.CopyFile(inputPath, resolvedPath)
	}

	state := tui.NewPhaseState([]string{"dnsx"})
	done := make(chan struct{})
	r.Start(state, done)

	state.Update("dnsx", func(ts *tui.ToolState) { ts.Status = tui.StatusRunning })
	if !r.IsTerminal() {
		r.PrintLine("[+] dnsx: running")
	}

	cmd := exec.CommandContext(ctx, "dnsx",
		"-l", inputPath,
		"-silent",
		"-t", strconv.Itoa(cfg.Threads),
		"-o", resolvedPath,
	)

	stderrW, cleanupStderr := verboseWriter(cfg.Verbose, state, "dnsx")
	cmd.Stderr = stderrW

	err := cmd.Run()
	cleanupStderr()

	count := normalize.CountLines(resolvedPath)
	state.Update("dnsx", func(ts *tui.ToolState) {
		if err != nil && ctx.Err() == nil {
			ts.Status = tui.StatusFailed
			ts.Message = err.Error()
		} else {
			ts.Status = tui.StatusDone
			ts.Count = count
		}
	})
	if !r.IsTerminal() {
		r.PrintLine("[✓] dnsx: %d results", count)
	}

	close(done)
	return nil
}
