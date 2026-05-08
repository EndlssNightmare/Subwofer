package phase

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"subwofer/internal/config"
	"subwofer/internal/runner"
	"subwofer/internal/tui"
)

// PassiveTools returns the full ordered list of passive enumeration tools.
func PassiveTools(cfg *config.Config) []runner.Runnable {
	return []runner.Runnable{
		buildSubfinder(),
		buildAmass(),
		buildFindomain(),
		buildAssetfinder(),
		buildChaos(),
		buildGithubSubdomains(),
		buildCrtsh(),
		buildWaybackurls(),
		buildGau(),
		buildHaktrails(),
		buildShodan(),
		buildBevigil(),
	}
}

// ─── subfinder ───────────────────────────────────────────────────────────────

func buildSubfinder() runner.Runnable {
	return &runner.CmdTool{
		Name: "subfinder",
		CheckAvail: func(cfg *config.Config) (string, error) {
			if !runner.HasCmd("subfinder") {
				return "not installed", nil
			}
			return "", nil
		},
		BuildCmds: func(ctx context.Context, cfg *config.Config) ([]*exec.Cmd, error) {
			args := []string{"-dL", cfg.DomainListFile(), "-silent"}
			if cfg.Keys.PDCP != "" {
				args = append(args, "-all")
			}
			cmd := exec.CommandContext(ctx, "subfinder", args...)
			cmd.Env = append(os.Environ(), "PDCP_API_KEY="+cfg.Keys.PDCP)
			return []*exec.Cmd{cmd}, nil
		},
	}
}

// ─── amass ───────────────────────────────────────────────────────────────────

func buildAmass() runner.Runnable {
	return &runner.CmdTool{
		Name: "amass",
		CheckAvail: func(cfg *config.Config) (string, error) {
			if !runner.HasCmd("amass") {
				return "not installed", nil
			}
			return "", nil
		},
		BuildCmds: func(ctx context.Context, cfg *config.Config) ([]*exec.Cmd, error) {
			tctx, _ := context.WithTimeout(ctx, 10*time.Minute)
			cmd := exec.CommandContext(tctx, "amass", "enum", "-passive", "-df", cfg.DomainListFile())
			return []*exec.Cmd{cmd}, nil
		},
	}
}

// ─── findomain ───────────────────────────────────────────────────────────────

func buildFindomain() runner.Runnable {
	return &runner.CmdTool{
		Name: "findomain",
		CheckAvail: func(cfg *config.Config) (string, error) {
			if !runner.HasCmd("findomain") {
				return "not installed", nil
			}
			return "", nil
		},
		BuildCmds: func(ctx context.Context, cfg *config.Config) ([]*exec.Cmd, error) {
			cmd := exec.CommandContext(ctx, "findomain", "--file", cfg.DomainListFile(), "--quiet")
			return []*exec.Cmd{cmd}, nil
		},
	}
}

// ─── assetfinder ─────────────────────────────────────────────────────────────

func buildAssetfinder() runner.Runnable {
	return &runner.CmdTool{
		Name: "assetfinder",
		CheckAvail: func(cfg *config.Config) (string, error) {
			if !runner.HasCmd("assetfinder") {
				return "not installed", nil
			}
			return "", nil
		},
		BuildCmds: func(ctx context.Context, cfg *config.Config) ([]*exec.Cmd, error) {
			var cmds []*exec.Cmd
			for _, d := range cfg.Domains {
				cmd := exec.CommandContext(ctx, "assetfinder", "--subs-only", d)
				cmds = append(cmds, cmd)
			}
			return cmds, nil
		},
	}
}

// ─── chaos ───────────────────────────────────────────────────────────────────

func buildChaos() runner.Runnable {
	return &runner.CmdTool{
		Name: "chaos",
		CheckAvail: func(cfg *config.Config) (string, error) {
			if !runner.HasCmd("chaos") {
				return "not installed", nil
			}
			if cfg.Keys.PDCP == "" {
				return "needs PDCP_KEY", nil
			}
			return "", nil
		},
		BuildCmds: func(ctx context.Context, cfg *config.Config) ([]*exec.Cmd, error) {
			var cmds []*exec.Cmd
			for _, d := range cfg.Domains {
				cmd := exec.CommandContext(ctx, "chaos", "-d", d, "-silent")
				cmd.Env = append(os.Environ(), "PDCP_API_KEY="+cfg.Keys.PDCP)
				cmds = append(cmds, cmd)
			}
			return cmds, nil
		},
	}
}

// ─── github-subdomains ───────────────────────────────────────────────────────

func buildGithubSubdomains() runner.Runnable {
	return &runner.CmdTool{
		Name: "github-subdomains",
		CheckAvail: func(cfg *config.Config) (string, error) {
			if !runner.HasCmd("github-subdomains") {
				return "not installed", nil
			}
			if cfg.Keys.GitHub == "" {
				return "needs GITHUB_TOKEN", nil
			}
			return "", nil
		},
		BuildCmds: func(ctx context.Context, cfg *config.Config) ([]*exec.Cmd, error) {
			var cmds []*exec.Cmd
			for _, d := range cfg.Domains {
				cmd := exec.CommandContext(ctx, "github-subdomains",
					"-d", d, "-t", cfg.Keys.GitHub, "-o", "/dev/stdout")
				cmds = append(cmds, cmd)
			}
			return cmds, nil
		},
	}
}

// ─── crt.sh (HTTP) ───────────────────────────────────────────────────────────

type crtEntry struct {
	NameValue string `json:"name_value"`
}

func buildCrtsh() runner.Runnable {
	return &runner.HTTPTool{
		Name:       "crtsh",
		CheckAvail: nil, // always available
		Fetch: func(ctx context.Context, cfg *config.Config) ([]string, error) {
			seen := make(map[string]struct{})
			var results []string

			client := &http.Client{}

			for _, d := range cfg.Domains {
				url := fmt.Sprintf("https://crt.sh/?q=%%25.%s&output=json", d)
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
				if err != nil {
					continue
				}
				resp, err := client.Do(req)
				if err != nil {
					continue
				}

				body, err := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if err != nil {
					continue
				}

				var entries []crtEntry
				if err := json.Unmarshal(body, &entries); err != nil {
					continue
				}

				for _, e := range entries {
					// name_value may contain multiple entries separated by newlines or commas.
					for _, raw := range strings.FieldsFunc(e.NameValue, func(r rune) bool {
						return r == '\n' || r == ','
					}) {
						raw = strings.TrimSpace(raw)
						raw = strings.TrimPrefix(raw, "*.")
						raw = strings.ToLower(raw)
						if raw == "" {
							continue
						}
						if _, ok := seen[raw]; !ok {
							seen[raw] = struct{}{}
							results = append(results, raw)
						}
					}
				}
			}
			return results, nil
		},
	}
}

// ─── waybackurls ─────────────────────────────────────────────────────────────

func buildWaybackurls() runner.Runnable {
	return &waybackTool{}
}

type waybackTool struct{}

func (w *waybackTool) GetName() string { return "waybackurls" }

func (w *waybackTool) IsAvailable(cfg *config.Config) (string, error) {
	if !runner.HasCmd("waybackurls") {
		return "not installed", nil
	}
	return "", nil
}

func (w *waybackTool) Run(ctx context.Context, cfg *config.Config, outPath string, logCh chan<- string) (int, error) {
	f, err := os.Create(outPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	bw := bufio.NewWriter(f)
	count := 0

	for _, d := range cfg.Domains {
		re := regexp.MustCompile(`(?i)[a-zA-Z0-9._-]+\.` + regexp.QuoteMeta(d))

		cmd := exec.CommandContext(ctx, "waybackurls")
		cmd.Stdin = strings.NewReader(d)

		if logCh != nil && cfg.Verbose {
			pr, pw := io.Pipe()
			cmd.Stderr = pw
			go func(r io.ReadCloser, p *io.PipeWriter) {
				defer p.Close()
				scanner := bufio.NewScanner(r)
				for scanner.Scan() {
					select {
					case logCh <- scanner.Text():
					default:
					}
				}
			}(pr, pw)
		} else {
			cmd.Stderr = io.Discard
		}

		out, err := cmd.Output()
		if err != nil && ctx.Err() != nil {
			return count, ctx.Err()
		}

		seen := make(map[string]struct{})
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			line := scanner.Text()
			matches := re.FindAllString(line, -1)
			for _, m := range matches {
				m = strings.ToLower(m)
				if _, ok := seen[m]; !ok {
					seen[m] = struct{}{}
					fmt.Fprintln(bw, m)
					count++
				}
			}
		}
	}

	return count, bw.Flush()
}

// ─── gau ─────────────────────────────────────────────────────────────────────

func buildGau() runner.Runnable {
	return &gauTool{}
}

type gauTool struct{}

func (g *gauTool) GetName() string { return "gau" }

func (g *gauTool) IsAvailable(cfg *config.Config) (string, error) {
	if !runner.HasCmd("gau") {
		return "not installed", nil
	}
	return "", nil
}

func (g *gauTool) Run(ctx context.Context, cfg *config.Config, outPath string, logCh chan<- string) (int, error) {
	f, err := os.Create(outPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	bw := bufio.NewWriter(f)
	count := 0

	for _, d := range cfg.Domains {
		re := regexp.MustCompile(`(?i)[a-zA-Z0-9._-]+\.` + regexp.QuoteMeta(d))

		cmd := exec.CommandContext(ctx, "gau", "--subs")
		cmd.Stdin = strings.NewReader(d)

		// Filter "error reading config" from stderr.
		if logCh != nil && cfg.Verbose {
			pr, pw := io.Pipe()
			cmd.Stderr = pw
			go func(r io.ReadCloser, p *io.PipeWriter) {
				defer p.Close()
				scanner := bufio.NewScanner(r)
				for scanner.Scan() {
					line := scanner.Text()
					if !strings.Contains(line, "error reading config") {
						select {
						case logCh <- line:
						default:
						}
					}
				}
			}(pr, pw)
		} else {
			cmd.Stderr = io.Discard
		}

		out, err := cmd.Output()
		if err != nil && ctx.Err() != nil {
			return count, ctx.Err()
		}

		seen := make(map[string]struct{})
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			line := scanner.Text()
			matches := re.FindAllString(line, -1)
			for _, m := range matches {
				m = strings.ToLower(m)
				if _, ok := seen[m]; !ok {
					seen[m] = struct{}{}
					fmt.Fprintln(bw, m)
					count++
				}
			}
		}
	}

	return count, bw.Flush()
}

// ─── haktrails ────────────────────────────────────────────────────────────────

func buildHaktrails() runner.Runnable {
	return &runner.CmdTool{
		Name: "haktrails",
		CheckAvail: func(cfg *config.Config) (string, error) {
			if !runner.HasCmd("haktrails") {
				return "not installed", nil
			}
			if cfg.Keys.PDCP == "" {
				return "needs PDCP_KEY", nil
			}
			return "", nil
		},
		BuildCmds: func(ctx context.Context, cfg *config.Config) ([]*exec.Cmd, error) {
			var cmds []*exec.Cmd
			for _, d := range cfg.Domains {
				cmd := exec.CommandContext(ctx, "haktrails", "subdomains")
				cmd.Stdin = strings.NewReader(d)
				cmd.Env = append(os.Environ(), "PDCP_API_KEY="+cfg.Keys.PDCP)
				cmds = append(cmds, cmd)
			}
			return cmds, nil
		},
	}
}

// ─── shodan ───────────────────────────────────────────────────────────────────

func buildShodan() runner.Runnable {
	return &shodanTool{}
}

type shodanTool struct{}

func (s *shodanTool) GetName() string { return "shodan" }

func (s *shodanTool) IsAvailable(cfg *config.Config) (string, error) {
	if !runner.HasCmd("shodan") {
		return "not installed", nil
	}
	if cfg.Keys.Shodan == "" {
		return "needs SHODAN_KEY", nil
	}
	return "", nil
}

func (s *shodanTool) Run(ctx context.Context, cfg *config.Config, outPath string, logCh chan<- string) (int, error) {
	f, err := os.Create(outPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	bw := bufio.NewWriter(f)
	count := 0

	for _, d := range cfg.Domains {
		re := regexp.MustCompile(`(?i)[a-zA-Z0-9._-]+\.` + regexp.QuoteMeta(d))

		cmd := exec.CommandContext(ctx, "shodan", "domain", d)
		cmd.Env = append(os.Environ(), "PYTHONWARNINGS=ignore")

		if logCh != nil && cfg.Verbose {
			pr, pw := io.Pipe()
			cmd.Stderr = pw
			go func(r io.ReadCloser, p *io.PipeWriter) {
				defer p.Close()
				scanner := bufio.NewScanner(r)
				for scanner.Scan() {
					select {
					case logCh <- scanner.Text():
					default:
					}
				}
			}(pr, pw)
		} else {
			cmd.Stderr = io.Discard
		}

		out, err := cmd.Output()
		if err != nil && ctx.Err() != nil {
			return count, ctx.Err()
		}

		seen := make(map[string]struct{})
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			line := scanner.Text()
			matches := re.FindAllString(line, -1)
			for _, m := range matches {
				m = strings.ToLower(m)
				if _, ok := seen[m]; !ok {
					seen[m] = struct{}{}
					fmt.Fprintln(bw, m)
					count++
				}
			}
		}
	}

	return count, bw.Flush()
}

// ─── bevigil ─────────────────────────────────────────────────────────────────

func buildBevigil() runner.Runnable {
	return &bevigilTool{}
}

type bevigilTool struct{}

func (b *bevigilTool) GetName() string { return "bevigil-cli" }

func (b *bevigilTool) IsAvailable(cfg *config.Config) (string, error) {
	if !runner.HasCmd("bevigil-cli") {
		return "not installed", nil
	}
	if cfg.Keys.Bevigil == "" {
		return "needs BEVIGIL_KEY", nil
	}
	return "", nil
}

func (b *bevigilTool) Run(ctx context.Context, cfg *config.Config, outPath string, logCh chan<- string) (int, error) {
	// Initialise bevigil-cli with the API key first.
	initCmd := exec.CommandContext(ctx, "bevigil-cli", "init", "--api-key", cfg.Keys.Bevigil)
	_ = initCmd.Run()

	f, err := os.Create(outPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	bw := bufio.NewWriter(f)
	count := 0
	seen := make(map[string]struct{})

	type bevigilResp struct {
		Subdomains []string `json:"subdomains"`
	}

	for _, d := range cfg.Domains {
		if ctx.Err() != nil {
			break
		}

		cmd := exec.CommandContext(ctx, "bevigil-cli", "enum", "subdomains", "--domain", d)

		if logCh != nil && cfg.Verbose {
			pr, pw := io.Pipe()
			cmd.Stderr = pw
			go func(r io.ReadCloser, p *io.PipeWriter) {
				defer p.Close()
				scanner := bufio.NewScanner(r)
				for scanner.Scan() {
					select {
					case logCh <- scanner.Text():
					default:
					}
				}
			}(pr, pw)
		} else {
			cmd.Stderr = io.Discard
		}

		out, err := cmd.Output()
		if err != nil {
			continue
		}

		var resp bevigilResp
		if err := json.Unmarshal(out, &resp); err != nil {
			// Fallback: try to parse line-by-line in case output is plain text.
			scanner := bufio.NewScanner(strings.NewReader(string(out)))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" {
					continue
				}
				if _, ok := seen[line]; !ok {
					seen[line] = struct{}{}
					fmt.Fprintln(bw, line)
					count++
				}
			}
			continue
		}

		for _, sub := range resp.Subdomains {
			sub = strings.TrimSpace(sub)
			if sub == "" {
				continue
			}
			if _, ok := seen[sub]; !ok {
				seen[sub] = struct{}{}
				fmt.Fprintln(bw, sub)
				count++
			}
		}
	}

	return count, bw.Flush()
}

// ─── RunPassive ───────────────────────────────────────────────────────────────

// RunPassive runs all passive tools concurrently, then aggregates results.
func RunPassive(ctx context.Context, cfg *config.Config, r *tui.Renderer) error {
	tools := PassiveTools(cfg)

	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.GetName()
	}

	state := tui.NewPhaseState(names)
	done := make(chan struct{})
	r.PrintPhaseHeader("Phase 1 — Passive Enumeration")
	r.Start(state, done)

	var wg sync.WaitGroup

	for _, tool := range tools {
		skipReason, err := tool.IsAvailable(cfg)
		if err != nil {
			state.Update(tool.GetName(), func(ts *tui.ToolState) {
				ts.Status = tui.StatusFailed
				ts.Message = err.Error()
			})
			if !r.IsTerminal() {
				snap := tui.ToolState{Name: tool.GetName(), Status: tui.StatusFailed, Message: err.Error()}
				r.PrintNonTTYUpdate(snap)
			}
			continue
		}
		if skipReason != "" {
			state.Update(tool.GetName(), func(ts *tui.ToolState) {
				ts.Status = tui.StatusSkipped
				ts.Message = skipReason
			})
			if !r.IsTerminal() {
				snap := tui.ToolState{Name: tool.GetName(), Status: tui.StatusSkipped, Message: skipReason}
				r.PrintNonTTYUpdate(snap)
			}
			continue
		}

		wg.Add(1)
		go func(t runner.Runnable) {
			defer wg.Done()

			state.Update(t.GetName(), func(ts *tui.ToolState) {
				ts.Status = tui.StatusRunning
			})
			if !r.IsTerminal() {
				snap := tui.ToolState{Name: t.GetName(), Status: tui.StatusRunning}
				r.PrintNonTTYUpdate(snap)
			}

			outPath := filepath.Join(cfg.SourcesDir, t.GetName()+".txt")

			var logCh chan string
			if cfg.Verbose {
				logCh = make(chan string, 100)
				go func() {
					for line := range logCh {
						state.AddLog(t.GetName(), line)
					}
				}()
			}

			count, err := t.Run(ctx, cfg, outPath, logCh)
			if logCh != nil {
				close(logCh)
			}

			state.Update(t.GetName(), func(ts *tui.ToolState) {
				if err != nil && ctx.Err() == nil {
					ts.Status = tui.StatusFailed
					ts.Message = err.Error()
				} else {
					ts.Status = tui.StatusDone
					ts.Count = count
				}
			})

			if !r.IsTerminal() {
				var snap tui.ToolState
				if err != nil && ctx.Err() == nil {
					snap = tui.ToolState{Name: t.GetName(), Status: tui.StatusFailed, Message: err.Error()}
				} else {
					snap = tui.ToolState{Name: t.GetName(), Status: tui.StatusDone, Count: count}
				}
				r.PrintNonTTYUpdate(snap)
			}
		}(tool)
	}

	wg.Wait()
	close(done)

	rawPath := filepath.Join(cfg.TmpDir, "all_raw.txt")
	return runner.AggregateSourceFiles(cfg, rawPath)
}
