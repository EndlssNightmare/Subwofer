package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"subwofer/internal/config"
	"subwofer/internal/normalize"
	"subwofer/internal/output"
	"subwofer/internal/phase"
	"subwofer/internal/tui"
)

const banner = `
 ███████╗██╗   ██╗██████╗ ██╗    ██╗ ██████╗ ███████╗███████╗██████╗
 ██╔════╝██║   ██║██╔══██╗██║    ██║██╔═══██╗██╔════╝██╔════╝██╔══██╗
 ███████╗██║   ██║██████╔╝██║ █╗ ██║██║   ██║█████╗  █████╗  ██████╔╝
 ╚════██║██║   ██║██╔══██╗██║███╗██║██║   ██║██╔══╝  ██╔══╝  ██╔══██╗
 ███████║╚██████╔╝██████╔╝╚███╔███╔╝╚██████╔╝██║     ███████╗██║  ██║
 ╚══════╝ ╚═════╝ ╚═════╝  ╚══╝╚══╝  ╚═════╝ ╚═╝     ╚══════╝╚═╝  ╚═╝
`

func main() {
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m[!]\033[0m %v\n", err)
		os.Exit(1)
	}

	cfg.Keys = config.LoadAPIKeys()

	if err := cfg.LoadDomains(); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m[!]\033[0m %v\n", err)
		os.Exit(1)
	}

	cfg.SetupPaths()
	cfg.Wordlist = config.DetectWordlist(cfg.Wordlist)

	// Write domain list file for tools that accept -dL.
	_ = cfg.DomainListFile()

	printBanner(cfg)

	// Context with cancellation for signal handling.
	ctx, cancel := context.WithCancel(context.Background())

	r := tui.NewRenderer(cfg.Verbose)

	// Handle SIGINT / SIGTERM gracefully.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		fmt.Fprintln(os.Stderr, "\n\033[31m[!]\033[0m Interrupted — cleaning up...")
		cancel()
		r.ShowCursor()
		_ = output.Cleanup(cfg.TmpDir)
		os.Exit(1)
	}()

	// ── Phase 1: Passive ──────────────────────────────────────────────────────
	if err := phase.RunPassive(ctx, cfg, r); err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "\033[31m[!]\033[0m passive phase error: %v\n", err)
	}

	// ── Phase 2: Bruteforce ───────────────────────────────────────────────────
	if !cfg.SkipBrute {
		if err := phase.RunBruteforce(ctx, cfg, r); err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "\033[31m[!]\033[0m bruteforce phase error: %v\n", err)
		}
	}

	// ── Phase 3: Permutation ──────────────────────────────────────────────────
	if !cfg.SkipPerm {
		if err := phase.RunPermutation(ctx, cfg, r); err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "\033[31m[!]\033[0m permutation phase error: %v\n", err)
		}
	}

	// ── Normalize & deduplicate ───────────────────────────────────────────────
	rawPath := filepath.Join(cfg.TmpDir, "all_raw.txt")
	finalSubs := filepath.Join(cfg.OutDir, "subdomains.txt")

	r.PrintLine("\n\033[1;36m[*]\033[0m Merging and deduplicating all results...")
	totalSubs, err := normalize.NormalizeRaw(rawPath, cfg.Domains, finalSubs)
	if err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "\033[31m[!]\033[0m normalize error: %v\n", err)
	}
	r.PrintLine("\033[32m[✓]\033[0m %d unique subdomains → %s", totalSubs, finalSubs)

	// ── Phase 4: Resolve ──────────────────────────────────────────────────────
	if err := phase.RunResolve(ctx, cfg, r); err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "\033[31m[!]\033[0m resolve phase error: %v\n", err)
	}

	// ── Phase 5: HTTP probing ─────────────────────────────────────────────────
	if !cfg.SkipHTTP {
		if err := phase.RunHTTP(ctx, cfg, r); err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "\033[31m[!]\033[0m http phase error: %v\n", err)
		}
	}

	// ── Phase 6: Screenshots ──────────────────────────────────────────────────
	if cfg.Screenshots {
		if err := phase.RunScreenshots(ctx, cfg, r); err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "\033[31m[!]\033[0m screenshots phase error: %v\n", err)
		}
	}

	// ── Phase 7: Virtual Host Discovery ───────────────────────────────────────
	if cfg.VhostFuzz {
		if err := phase.RunVhosts(ctx, cfg, r); err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "\033[31m[!]\033[0m vhost phase error: %v\n", err)
		}
	}

	// ── Cleanup ───────────────────────────────────────────────────────────────
	r.PrintLine("\n\033[36m[*]\033[0m Cleaning up temporary files...")
	if err := output.Cleanup(cfg.TmpDir); err != nil {
		fmt.Fprintf(os.Stderr, "\033[33m[~]\033[0m cleanup warning: %v\n", err)
	}

	// ── Summary ───────────────────────────────────────────────────────────────
	printSummary(cfg, r)

	cancel() // tidy up context
}

// parseArgs parses command-line arguments into a Config without using pflag.
func parseArgs(args []string) (*config.Config, error) {
	cfg := &config.Config{Threads: 100, PermMaxBase: 300}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-d":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("-d requires an argument")
			}
			cfg.Domain = args[i]
		case "-dL":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("-dL requires an argument")
			}
			cfg.DomainFile = args[i]
		case "-o":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("-o requires an argument")
			}
			cfg.OutDir = args[i]
		case "-w":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("-w requires an argument")
			}
			cfg.Wordlist = args[i]
		case "-t":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("-t requires an argument")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return nil, fmt.Errorf("-t: invalid number: %s", args[i])
			}
			cfg.Threads = n
		case "-s":
			cfg.Screenshots = true
		case "-H":
			cfg.SkipHTTP = true
		case "-B":
			cfg.SkipBrute = true
		case "-P":
			cfg.SkipPerm = true
		case "-perm-max":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("-perm-max requires an argument")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return nil, fmt.Errorf("-perm-max: invalid number: %s", args[i])
			}
			cfg.PermMaxBase = n
		case "--perm-full":
			cfg.PermFull = true
		case "--vhost":
			cfg.VhostFuzz = true
		case "--vhost-wordlist":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("--vhost-wordlist requires an argument")
			}
			cfg.VhostWordlist = args[i]
		case "-v":
			cfg.Verbose = true
		case "-h", "--help":
			printHelp()
			os.Exit(0)
		default:
			return nil, fmt.Errorf("unknown flag: %s", args[i])
		}
	}

	return cfg, nil
}

// printHelp prints usage information.
func printHelp() {
	const (
		reset  = "\033[0m"
		bold   = "\033[1m"
		cyan   = "\033[36m"
		green  = "\033[32m"
		yellow = "\033[33m"
		dim    = "\033[2m"
	)

	row := func(flag, arg, desc, def string) {
		flagCol := fmt.Sprintf("%s%s%-12s%s", bold, cyan, flag, reset)
		argCol := fmt.Sprintf("%-10s", arg)
		defStr := ""
		if def != "" {
			defStr = fmt.Sprintf("  %s(default: %s)%s", dim, def, reset)
		}
		fmt.Printf("  %s %s%s%s\n", flagCol, argCol, desc, defStr)
	}

	fmt.Printf("\n%s%s  subwofer%s — Subdomain Enumeration Pipeline\n\n", bold, cyan, reset)

	fmt.Printf("%sUsage:%s\n", bold, reset)
	fmt.Printf("  subwofer -d <domain> [OPTIONS]\n")
	fmt.Printf("  subwofer -dL <file>  [OPTIONS]\n\n")

	fmt.Printf("%sInput%s %s(use one)%s\n", bold, reset, dim, reset)
	row("-d", "<domain>", "Single target domain", "")
	row("-dL", "<file>", "File with one domain per line", "")
	fmt.Println()

	fmt.Printf("%sOutput%s\n", bold, reset)
	row("-o", "<dir>", "Output directory", "./results/<domain>")
	fmt.Println()

	fmt.Printf("%sScan%s\n", bold, reset)
	row("-t", "<n>", "DNS/HTTP resolver threads", "100")
	row("-w", "<file>", "Wordlist for bruteforce", "auto (SecLists)")
	row("-s", "", "Take screenshots with gowitness", "")
	row("--vhost", "", "Virtual host discovery with ffuf (auto baseline filter)", "")
	row("--vhost-wordlist", "<file>", "Custom wordlist for vhost fuzzing", "built-in ~80 words")
	fmt.Println()

	fmt.Printf("%sSkip Phases%s\n", bold, reset)
	row("-H", "", "Skip HTTP probing (httpx)", "")
	row("-B", "", "Skip bruteforce phase", "")
	row("-P", "", "Skip permutation phase entirely", "")
	row("-perm-max", "<n>", "Cap permutation base size (0 = unlimited)", "300")
	row("--perm-full", "", "Also run gotator+altdns (slower, more candidates)", "")
	fmt.Println()

	fmt.Printf("%sMisc%s\n", bold, reset)
	row("-v", "", "Verbose — show tool stderr output", "")
	row("-h", "", "Show this help", "")
	fmt.Println()

	fmt.Printf("%sAPI Keys%s %s(env vars)%s\n", bold, reset, dim, reset)
	fmt.Printf("  %sGITHUB_TOKEN%s        GitHub passive enum\n", green, reset)
	fmt.Printf("  %sSECURITYTRAILS_KEY%s  SecurityTrails passive enum\n", green, reset)
	fmt.Printf("  %sSHODAN_KEY%s          Shodan passive enum\n", green, reset)
	fmt.Printf("  %sBEVIGIL_KEY%s         Bevigil passive enum\n", green, reset)
	fmt.Printf("  %sPDCP_KEY%s            ProjectDiscovery Cloud\n", green, reset)
	fmt.Println()

	fmt.Printf("%sExamples%s\n", bold, reset)
	fmt.Printf("  %s# Full scan, single domain%s\n", dim, reset)
	fmt.Printf("  subwofer -d example.com\n\n")
	fmt.Printf("  %s# Multi-domain with screenshots%s\n", dim, reset)
	fmt.Printf("  subwofer -dL targets.txt -s\n\n")
	fmt.Printf("  %s# Skip bruteforce + permutation (passive only)%s\n", dim, reset)
	fmt.Printf("  subwofer -d example.com -B -P\n\n")
	fmt.Printf("  %s# Large scope — skip perm or raise the cap%s\n", dim, reset)
	fmt.Printf("  subwofer -dL big_scope.txt -perm-max 0\n")
	fmt.Println()

	_ = yellow
}

// printBanner prints the ASCII art banner and configuration block.
func printBanner(cfg *config.Config) {
	const cyan = "\033[36m"
	const yellow = "\033[33m"
	const bold = "\033[1m"
	const red = "\033[31m"
	const green = "\033[32m"
	const reset = "\033[0m"

	fmt.Fprintf(os.Stderr, "%s%s%s%s\n", cyan, bold, banner, reset)

	// Determine target display string.
	target := cfg.Label
	if len(cfg.Domains) > 1 {
		target = fmt.Sprintf("%d domains", len(cfg.Domains))
	}

	wordlistDisplay := cfg.Wordlist
	if wordlistDisplay == "" {
		if cfg.SkipBrute {
			wordlistDisplay = "n/a (bruteforce skipped)"
		} else {
			wordlistDisplay = "built-in (1000 words)"
		}
	}

	// API key status.
	keyStatus := func(key, name string) string {
		if key != "" {
			return green + name + " ✓" + reset
		}
		return name + " -"
	}

	keySummary := fmt.Sprintf("%s  %s  %s  %s  %s",
		keyStatus(cfg.Keys.PDCP, "PDCP"),
		keyStatus(cfg.Keys.GitHub, "GitHub"),
		keyStatus(cfg.Keys.Shodan, "Shodan"),
		keyStatus(cfg.Keys.Bevigil, "Bevigil"),
		keyStatus(cfg.Keys.SecurityTrails, "SecurityTrails"),
	)

	fmt.Fprintf(os.Stderr, " ┌─────────────────────────────────────────────────┐\n")
	fmt.Fprintf(os.Stderr, " │  target   : %-35s│\n", target)
	fmt.Fprintf(os.Stderr, " │  output   : %-35s│\n", cfg.OutDir)
	fmt.Fprintf(os.Stderr, " │  wordlist : %-35s│\n", truncate(wordlistDisplay, 35))
	fmt.Fprintf(os.Stderr, " │  threads  : %-35d│\n", cfg.Threads)
	fmt.Fprintf(os.Stderr, " │  api keys : %-35s│\n", keySummary)
	fmt.Fprintf(os.Stderr, " └─────────────────────────────────────────────────┘\n\n")

	_ = yellow
	_ = bold
}

// printSummary prints the final results summary box.
func printSummary(cfg *config.Config, r *tui.Renderer) {
	finalSubs := filepath.Join(cfg.OutDir, "subdomains.txt")
	resolvedSubs := filepath.Join(cfg.OutDir, "subdomains_resolved.txt")
	liveURLs := filepath.Join(cfg.OutDir, "live_urls.txt")
	screenshotsDir := filepath.Join(cfg.OutDir, "screenshots")

	files := make(map[string]int)
	paths := make(map[string]string)

	if n := normalize.CountLines(finalSubs); n > 0 {
		files["all subdomains"] = n
		paths["all subdomains"] = finalSubs
	}
	if n := normalize.CountLines(resolvedSubs); n > 0 {
		files["resolved"] = n
		paths["resolved"] = resolvedSubs
	}
	if n := normalize.CountLines(liveURLs); n > 0 {
		files["live URLs"] = n
		paths["live URLs"] = liveURLs
	}
	vhostsFound := filepath.Join(cfg.OutDir, "vhosts_found.txt")
	if cfg.VhostFuzz {
		if n := normalize.CountLines(vhostsFound); n > 0 {
			files["vhosts"] = n
			paths["vhosts"] = vhostsFound
		}
	}
	if cfg.Screenshots {
		entries, _ := os.ReadDir(screenshotsDir)
		count := 0
		for _, e := range entries {
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if !e.IsDir() && (ext == ".png" || ext == ".jpg" || ext == ".jpeg") {
				count++
			}
		}
		if count > 0 || cfg.Screenshots {
			files["screenshots"] = count
			paths["screenshots"] = screenshotsDir
		}
	}

	r.PrintSummary(files, paths)
}

func truncate(s string, max int) string {
	// Strip ANSI codes for length calculation isn't trivial; just truncate raw.
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func hasSuffix(s, suffix string) bool {
	if len(s) < len(suffix) {
		return false
	}
	return s[len(s)-len(suffix):] == suffix
}
