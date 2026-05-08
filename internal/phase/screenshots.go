package phase

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"subwofer/internal/config"
	"subwofer/internal/normalize"
	"subwofer/internal/runner"
	"subwofer/internal/tui"
)

// RunScreenshots takes screenshots of live URLs using gowitness.
func RunScreenshots(ctx context.Context, cfg *config.Config, r *tui.Renderer) error {
	r.PrintPhaseHeader("Phase 6 — Screenshots")

	if !runner.HasCmd("gowitness") {
		r.PrintLine("  \033[33m~\033[0m gowitness not installed — skipping")
		return nil
	}

	liveURLs := filepath.Join(cfg.OutDir, "live_urls.txt")
	if normalize.CountLines(liveURLs) == 0 {
		r.PrintLine("  \033[33m~\033[0m No live URLs available for screenshots")
		return nil
	}

	screenshotsDir := filepath.Join(cfg.OutDir, "screenshots")
	_ = os.MkdirAll(screenshotsDir, 0o755)

	// Extract first column from httpx output (just the URL).
	urlsFile := filepath.Join(cfg.TmpDir, "urls_for_screenshots.txt")
	if err := extractFirstColumn(liveURLs, urlsFile); err != nil {
		return fmt.Errorf("extract URLs for screenshots: %w", err)
	}

	state := tui.NewPhaseState([]string{"gowitness"})
	done := make(chan struct{})
	r.Start(state, done)

	state.Update("gowitness", func(ts *tui.ToolState) { ts.Status = tui.StatusRunning })
	if !r.IsTerminal() {
		r.PrintLine("[+] gowitness: running")
	}

	// Try v3 API first; fall back to v2.
	v3Cmd := exec.CommandContext(ctx, "gowitness", "scan", "file",
		"-f", urlsFile,
		"--screenshot-path", screenshotsDir,
		"--write-none",
	)

	var runErr error
	v3Stderr, cleanupV3 := verboseWriter(cfg.Verbose, state, "gowitness")
	v3Cmd.Stderr = v3Stderr
	if err := v3Cmd.Run(); err != nil {
		cleanupV3()
		// v3 failed — try v2.
		v2Cmd := exec.CommandContext(ctx, "gowitness", "file",
			"-f", urlsFile,
			"--destination", screenshotsDir,
		)
		v2Stderr, cleanupV2 := verboseWriter(cfg.Verbose, state, "gowitness")
		v2Cmd.Stderr = v2Stderr
		runErr = v2Cmd.Run()
		cleanupV2()
	} else {
		cleanupV3()
	}

	// Count screenshots.
	entries, _ := os.ReadDir(screenshotsDir)
	count := 0
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !e.IsDir() && (ext == ".png" || ext == ".jpg" || ext == ".jpeg") {
			count++
		}
	}

	state.Update("gowitness", func(ts *tui.ToolState) {
		if runErr != nil && ctx.Err() == nil {
			ts.Status = tui.StatusFailed
			ts.Message = runErr.Error()
		} else {
			ts.Status = tui.StatusDone
			ts.Count = count
		}
	})
	if !r.IsTerminal() {
		r.PrintLine("[✓] gowitness: %d screenshots", count)
	}

	close(done)
	return nil
}

// extractFirstColumn reads src line by line and writes the first whitespace-
// separated field of each line to dst.
func extractFirstColumn(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	w := bufio.NewWriter(out)
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		fmt.Fprintln(w, fields[0])
	}
	return w.Flush()
}
