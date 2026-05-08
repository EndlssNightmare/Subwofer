package phase

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"subwofer/internal/config"
	"subwofer/internal/normalize"
	"subwofer/internal/runner"
	"subwofer/internal/tui"
)

// vhostBuiltinWordlist is a curated list of common virtual host names.
// Focused on high-value targets: staging, internal tools, admin panels, APIs.
var vhostBuiltinWordlist = strings.Join([]string{
	"api", "api2", "api-v1", "api-v2", "apis",
	"dev", "development", "staging", "stage", "stg",
	"test", "testing", "qa", "uat", "preprod", "pre-prod",
	"admin", "administrator", "panel", "cp", "controlpanel",
	"internal", "intranet", "corp", "corporate", "extranet",
	"beta", "alpha", "demo", "sandbox", "preview",
	"old", "new", "v1", "v2", "v3", "legacy",
	"app", "apps", "application", "web", "www", "www2",
	"mobile", "m", "wap",
	"mail", "smtp", "imap", "pop", "webmail", "email",
	"ftp", "sftp", "files", "upload", "uploads",
	"vpn", "remote", "rdp", "citrix",
	"git", "gitlab", "github", "bitbucket", "svn",
	"jenkins", "ci", "cd", "build", "deploy",
	"jira", "confluence", "wiki", "docs", "documentation",
	"grafana", "kibana", "elk", "monitoring", "metrics", "prometheus",
	"cdn", "static", "assets", "media", "images", "img", "img2",
	"backup", "backups", "archive",
	"db", "database", "mysql", "postgres", "redis", "mongo",
	"elasticsearch", "elastic", "solr",
	"auth", "login", "sso", "oauth", "id", "identity",
	"shop", "store", "ecommerce", "payment", "billing", "invoice",
	"support", "help", "helpdesk", "ticket",
	"crm", "erp", "hr", "payroll",
	"reports", "analytics", "stats", "dashboard",
	"beta-api", "dev-api", "staging-api", "test-api",
	"api-dev", "api-staging", "api-test",
	"microservice", "service", "services",
	"proxy", "gateway", "mgmt", "management",
}, "\n")

// vhostBaseline sends two GET requests with a random non-existent Host header
// and returns the stable response size and word count to use as ffuf filters.
// Returns (-1, -1) if the target is unreachable.
func vhostBaseline(targetURL, rootDomain string) (size int, words int) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // don't follow redirects — baseline must be consistent
		},
	}

	probe := func() (int, int, error) {
		b := make([]byte, 8)
		rand.Read(b)
		fakeHost := "bbpf-" + hex.EncodeToString(b) + "." + rootDomain

		req, err := http.NewRequest("GET", targetURL, nil)
		if err != nil {
			return 0, 0, err
		}
		req.Host = fakeHost
		resp, err := client.Do(req)
		if err != nil {
			return 0, 0, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
		if err != nil {
			return 0, 0, err
		}
		w := len(strings.Fields(string(body)))
		return len(body), w, nil
	}

	sz1, wc1, err1 := probe()
	if err1 != nil {
		return -1, -1
	}
	sz2, wc2, err2 := probe()
	if err2 != nil {
		return sz1, wc1 // use first probe if second fails
	}

	// If the two probes agree on size, use size filtering (most precise).
	// If they differ by more than 5%, fall back to word count.
	if sz1 == sz2 {
		return sz1, -1 // signal: filter by size
	}
	diff := sz1 - sz2
	if diff < 0 {
		diff = -diff
	}
	if diff*100/max(sz1, 1) < 5 {
		return sz1, -1 // close enough — filter by size (use first)
	}
	// Dynamic body (timestamps, tokens) → filter by word count instead
	return -1, min(wc1, wc2)
}

// rootDomainOf extracts the registrable domain from a host (last 2 labels).
// "api.staging.example.com" → "example.com"
func rootDomainOf(host string) string {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	parts := strings.Split(host, ".")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], ".")
	}
	return host
}

// baseURLOf extracts scheme+host from a live_urls.txt line (first whitespace field).
func baseURLOf(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	u := fields[0]
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return ""
	}
	// Reconstruct scheme + host only (strip path if any)
	trimmed := strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
	hostPart := strings.SplitN(trimmed, "/", 2)[0]
	if strings.HasPrefix(u, "https://") {
		return "https://" + hostPart
	}
	return "http://" + hostPart
}

// hostOf extracts just the hostname (no port, no scheme) from a URL-like string.
func hostOf(u string) string {
	trimmed := strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
	host := strings.SplitN(trimmed, "/", 2)[0]
	host = strings.SplitN(host, ":", 2)[0]
	return strings.ToLower(host)
}

// linesFromPath reads non-empty, non-comment lines from a file.
func linesFromPath(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l != "" && !strings.HasPrefix(l, "#") {
			out = append(out, l)
		}
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RunVhosts runs virtual host discovery using ffuf against each unique live host.
// It probes a random non-existent vhost first to establish a baseline response,
// then runs ffuf filtering that baseline so only real vhosts surface.
func RunVhosts(ctx context.Context, cfg *config.Config, r *tui.Renderer) error {
	r.PrintPhaseHeader("Phase 7 — Virtual Host Discovery")

	if !runner.HasCmd("ffuf") {
		r.PrintLine("  \033[33m~\033[0m ffuf not installed — skipping vhost fuzzing")
		return nil
	}

	liveURLsPath := filepath.Join(cfg.OutDir, "live_urls.txt")
	if normalize.CountLines(liveURLsPath) == 0 {
		r.PrintLine("  \033[33m~\033[0m No live URLs — skipping vhost fuzzing")
		return nil
	}

	// Write wordlist.
	wl := cfg.VhostWordlist
	if wl == "" {
		builtinPath := filepath.Join(cfg.TmpDir, "vhost_builtin.txt")
		if err := os.WriteFile(builtinPath, []byte(vhostBuiltinWordlist), 0o644); err != nil {
			r.PrintLine("  \033[33m~\033[0m could not write vhost wordlist — skipping")
			return nil
		}
		wl = builtinPath
	}

	// Collect one base URL per root domain (deduplicate).
	// httpx output format: "https://host [200] [Title] [tech]" — use first field.
	seenRoot := map[string]string{} // rootDomain → baseURL
	for _, line := range linesFromPath(liveURLsPath) {
		base := baseURLOf(line)
		if base == "" {
			continue
		}
		root := rootDomainOf(hostOf(base))
		if _, exists := seenRoot[root]; !exists {
			seenRoot[root] = base
		}
	}

	if len(seenRoot) == 0 {
		r.PrintLine("  \033[33m~\033[0m No valid hosts parsed — skipping vhost fuzzing")
		return nil
	}

	vhostOut := filepath.Join(cfg.OutDir, "vhosts_found.txt")
	outF, _ := os.Create(vhostOut)
	if outF != nil {
		defer outF.Close()
	}

	state := tui.NewPhaseState([]string{"ffuf-vhost"})
	done := make(chan struct{})
	r.Start(state, done)
	state.Update("ffuf-vhost", func(ts *tui.ToolState) { ts.Status = tui.StatusRunning })
	if !r.IsTerminal() {
		r.PrintLine("[+] ffuf-vhost: running (%d root domain(s))", len(seenRoot))
	}

	totalFound := 0

	for rootDomain, baseURL := range seenRoot {
		if ctx.Err() != nil {
			break
		}

		// Establish baseline — what does the server return for a non-existent host?
		sz, wc := vhostBaseline(baseURL, rootDomain)
		if sz == -1 && wc == -1 {
			// Server not responding to fake hosts at all — skip, nothing to fuzz.
			continue
		}

		// Build ffuf args.
		// -s = silent mode: only print matched FUZZ values to stdout.
		// -mc all = match all status codes (we filter by response characteristics, not status).
		args := []string{
			"-u", baseURL,
			"-H", "Host: FUZZ." + rootDomain,
			"-w", wl,
			"-mc", "all",
			"-t", "40",
			"-timeout", "10",
			"-s", // silent — only found vhosts go to stdout
		}

		if sz >= 0 {
			// Filter by exact response size (most precise).
			args = append(args, "-fs", strconv.Itoa(sz))
		} else {
			// Dynamic body — filter by word count instead.
			args = append(args, "-fw", strconv.Itoa(wc))
		}

		var stdout bytes.Buffer
		cmd := exec.CommandContext(ctx, "ffuf", args...)
		cmd.Stdout = &stdout
		stderrW, cleanup := verboseWriter(cfg.Verbose, state, "ffuf-vhost")
		cmd.Stderr = stderrW
		_ = cmd.Run()
		cleanup()

		// Each line is a matched FUZZ word (the vhost prefix that got a different response).
		for _, match := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
			match = strings.TrimSpace(match)
			if match == "" {
				continue
			}
			vhost := match + "." + rootDomain
			r.PrintLine("  \033[32m[VHOST]\033[0m %s (server: %s)", vhost, baseURL)
			if outF != nil {
				fmt.Fprintln(outF, vhost)
			}
			totalFound++
		}
	}

	state.Update("ffuf-vhost", func(ts *tui.ToolState) {
		ts.Status = tui.StatusDone
		ts.Count = totalFound
	})
	if !r.IsTerminal() {
		r.PrintLine("[✓] ffuf-vhost: %d virtual host(s) found", totalFound)
	}
	close(done)
	return nil
}
