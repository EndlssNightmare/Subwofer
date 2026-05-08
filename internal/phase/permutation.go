package phase

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"

	"subwofer/internal/config"
	"subwofer/internal/normalize"
	"subwofer/internal/output"
	"subwofer/internal/runner"
	"subwofer/internal/tui"
	"subwofer/pkg/executil"
)

// permWords is a focused ~80-word list of meaningful subdomain mutations.
// Intentionally small: 200 subs × 80 words = 16k candidates, not 140k.
var permWords = []string{
	// environments
	"dev", "development", "staging", "stg", "stage",
	"prod", "production", "preprod", "pre-prod", "pre",
	"test", "testing", "qa", "uat", "sandbox",
	"demo", "beta", "alpha", "preview", "canary",
	// versions
	"v1", "v2", "v3", "v4",
	"old", "new", "legacy", "next", "latest",
	// regions
	"us", "eu", "ap", "uk", "de", "fr", "sg", "au",
	"us-east", "us-west", "eu-west", "eu-central", "ap-southeast",
	"global", "int", "internal",
	// function / role
	"api", "api2", "rest", "grpc",
	"admin", "management", "manage", "console", "dashboard", "portal",
	"web", "app", "mobile",
	"auth", "login", "sso", "oauth",
	"static", "assets", "cdn", "media", "img", "images",
	"support", "help", "docs", "status",
	// infra
	"gateway", "gw", "proxy", "lb",
	"backend", "frontend", "service", "svc",
	"worker", "jobs", "cron",
	"db", "data", "analytics",
	"mail", "smtp", "mx",
	// misc
	"corp", "private", "public", "secure", "ext", "external",
	"backup", "mirror", "2", "3",
}

// RunPermutation runs alterx and gotator in parallel against the current set
// of discovered subdomains, then appends results to all_raw.txt.
func RunPermutation(ctx context.Context, cfg *config.Config, r *tui.Renderer) error {
	r.PrintPhaseHeader("Phase 3 — Permutation & Alteration")

	rawPath := filepath.Join(cfg.TmpDir, "all_raw.txt")
	subsForPerm := filepath.Join(cfg.TmpDir, "subs_for_perm.txt")

	// Snapshot current unique subs as permutation input.
	count, err := normalize.NormalizeRaw(rawPath, cfg.Domains, subsForPerm)
	if err != nil {
		return err
	}
	r.PrintLine("    Using %d subdomains as permutation base", count)

	// Cap the permutation base to avoid combinatorial explosion.
	if cfg.PermMaxBase > 0 && count > cfg.PermMaxBase {
		r.PrintLine("  \033[33m~\033[0m base > %d — truncating to %d (use -perm-max 0 to disable)", cfg.PermMaxBase, cfg.PermMaxBase)
		if err := truncateFile(subsForPerm, cfg.PermMaxBase); err != nil {
			r.PrintLine("  \033[31m!\033[0m could not truncate perm base: %v", err)
		}
		count = cfg.PermMaxBase
	}

	hasDnsx := runner.HasCmd("dnsx")
	hasAlterx := runner.HasCmd("alterx")
	// gotator and altdns are heavy (wordlist × base = tens of thousands of DNS queries).
	// Only enable them with --perm-full; default uses only alterx (smart, pattern-based).
	hasGotator := cfg.PermFull && runner.HasCmd("gotator")
	hasAltdns := cfg.PermFull && runner.HasCmd("altdns")

	if !hasAlterx && !hasGotator && !hasAltdns {
		r.PrintLine("  \033[33m~\033[0m alterx not installed — skipping")
		return nil
	}

	// Collect tool names for display.
	var toolNames []string
	if hasAlterx {
		toolNames = append(toolNames, "alterx")
	}
	if hasGotator {
		toolNames = append(toolNames, "gotator")
	}
	if hasAltdns {
		toolNames = append(toolNames, "altdns")
	}

	// Write focused permutation wordlist (only needed when gotator/altdns are active).
	var permWordlistPath string
	if hasGotator || hasAltdns {
		permWordlistPath = filepath.Join(cfg.TmpDir, "perm_words.txt")
		if err := writePermWordlist(permWordlistPath); err != nil {
			r.PrintLine("  \033[31m!\033[0m could not write perm wordlist: %v", err)
			return err
		}
		r.PrintLine("    Full permutation: gotator+altdns enabled (~%d candidates/tool)", count*len(permWords))
	}

	state := tui.NewPhaseState(toolNames)
	spinDone := make(chan struct{})
	r.Start(state, spinDone)

	threads := strconv.Itoa(cfg.Threads)

	var wg sync.WaitGroup

	if hasAlterx {
		wg.Add(1)
		go func() {
			defer wg.Done()
			state.Update("alterx", func(ts *tui.ToolState) { ts.Status = tui.StatusRunning })

			outPath := filepath.Join(cfg.SourcesDir, "alterx.txt")
			var cmds []*exec.Cmd
			catCmd := exec.CommandContext(ctx, "cat", subsForPerm)

			if hasDnsx {
				alterxCmd := exec.CommandContext(ctx, "alterx", "-silent")
				dnsxCmd := exec.CommandContext(ctx, "dnsx", "-silent", "-t", threads)
				cmds = []*exec.Cmd{catCmd, alterxCmd, dnsxCmd}
			} else {
				alterxCmd := exec.CommandContext(ctx, "alterx", "-silent")
				cmds = []*exec.Cmd{catCmd, alterxCmd}
			}

			stderrW, cleanupStderr := verboseWriter(cfg.Verbose, state, "alterx")
			n, err := executil.PipeCommands(ctx, cmds, outPath, stderrW)
			cleanupStderr()
			state.Update("alterx", func(ts *tui.ToolState) {
				if err != nil && ctx.Err() == nil {
					ts.Status = tui.StatusFailed
					ts.Message = err.Error()
				} else {
					ts.Status = tui.StatusDone
					ts.Count = n
				}
			})
		}()
	}

	if hasGotator {
		wg.Add(1)
		go func() {
			defer wg.Done()
			state.Update("gotator", func(ts *tui.ToolState) { ts.Status = tui.StatusRunning })

			outPath := filepath.Join(cfg.SourcesDir, "gotator.txt")
			var cmds []*exec.Cmd

			// Use our focused wordlist via -perm instead of gotator's default 700-word list.
			gotatorCmd := exec.CommandContext(ctx, "gotator",
				"-sub", subsForPerm,
				"-perm", permWordlistPath,
				"-silent", "-depth", "1")
			if hasDnsx {
				dnsxCmd := exec.CommandContext(ctx, "dnsx", "-silent", "-t", threads)
				cmds = []*exec.Cmd{gotatorCmd, dnsxCmd}
			} else {
				cmds = []*exec.Cmd{gotatorCmd}
			}

			stderrW, cleanupStderr := verboseWriter(cfg.Verbose, state, "gotator")
			n, err := executil.PipeCommands(ctx, cmds, outPath, stderrW)
			cleanupStderr()
			state.Update("gotator", func(ts *tui.ToolState) {
				if err != nil && ctx.Err() == nil {
					ts.Status = tui.StatusFailed
					ts.Message = err.Error()
				} else {
					ts.Status = tui.StatusDone
					ts.Count = n
				}
			})
		}()
	}

	if hasAltdns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			state.Update("altdns", func(ts *tui.ToolState) { ts.Status = tui.StatusRunning })

			altdnsRaw := filepath.Join(cfg.TmpDir, "altdns_raw.txt")
			finalOut := filepath.Join(cfg.SourcesDir, "altdns.txt")

			// altdns requires a wordlist; use our focused one.
			altdnsCmd := exec.CommandContext(ctx, "altdns",
				"-i", subsForPerm,
				"-w", permWordlistPath,
				"-o", altdnsRaw,
			)
			altdnsStderr, cleanupAltdns := verboseWriter(cfg.Verbose, state, "altdns")
			altdnsCmd.Stderr = altdnsStderr
			_ = altdnsCmd.Run()
			cleanupAltdns()

			if _, err := os.Stat(altdnsRaw); os.IsNotExist(err) {
				state.Update("altdns", func(ts *tui.ToolState) {
					ts.Status = tui.StatusFailed
					ts.Message = "altdns produced no output"
				})
				return
			}

			if hasDnsx {
				dnsxCmd := exec.CommandContext(ctx, "dnsx", "-l", altdnsRaw, "-silent", "-t", threads, "-o", finalOut)
				dnsxStderr, cleanupDnsx := verboseWriter(cfg.Verbose, state, "altdns")
				dnsxCmd.Stderr = dnsxStderr
				_ = dnsxCmd.Run()
				cleanupDnsx()
			} else {
				_ = output.CopyFile(altdnsRaw, finalOut)
			}

			n := normalize.CountLines(finalOut)
			state.Update("altdns", func(ts *tui.ToolState) {
				ts.Status = tui.StatusDone
				ts.Count = n
			})
		}()
	}

	wg.Wait()
	close(spinDone)

	// Aggregate all permutation results into all_raw.txt.
	for _, name := range toolNames {
		src := filepath.Join(cfg.SourcesDir, name+".txt")
		if _, err := os.Stat(src); err == nil {
			_ = output.AppendToFile(rawPath, src)
		}
	}

	return nil
}

// writePermWordlist writes the embedded permWords slice to path.
func writePermWordlist(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, word := range permWords {
		fmt.Fprintln(w, word)
	}
	return w.Flush()
}

// truncateFile rewrites path keeping only the first n non-empty lines.
func truncateFile(path string, n int) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() && len(lines) < n {
		if l := sc.Text(); l != "" {
			lines = append(lines, l)
		}
	}
	f.Close()

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	bw := bufio.NewWriter(out)
	for _, l := range lines {
		fmt.Fprintln(bw, l)
	}
	return bw.Flush()
}
