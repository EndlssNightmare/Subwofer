package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds all runtime configuration.
type Config struct {
	Domain, DomainFile string
	Domains            []string
	Label              string
	OutDir, TmpDir, SourcesDir string
	Wordlist           string
	Threads            int
	PermMaxBase                                          int
	Screenshots, SkipHTTP, SkipBrute, SkipPerm, Verbose bool
	PermFull                                             bool // use gotator+altdns in addition to alterx
	VhostFuzz    bool   // run virtual host discovery with ffuf after HTTP probing
	VhostWordlist string // custom wordlist for vhost fuzzing (falls back to built-in)
	Keys               APIKeys
}

// APIKeys holds optional API credentials.
type APIKeys struct {
	GitHub, Shodan, Bevigil, PDCP, SecurityTrails string
}

// LoadAPIKeys reads API keys from environment variables.
// Shodan also falls back to ~/.config/shodan/api_key and ~/.shodan/api_key.
func LoadAPIKeys() APIKeys {
	k := APIKeys{
		GitHub:         os.Getenv("GITHUB_TOKEN"),
		Bevigil:        os.Getenv("BEVIGIL_KEY"),
		PDCP:           os.Getenv("PDCP_KEY"),
		SecurityTrails: os.Getenv("SECURITYTRAILS_KEY"),
		Shodan:         os.Getenv("SHODAN_KEY"),
	}

	if k.Shodan == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			candidates := []string{
				filepath.Join(home, ".config", "shodan", "api_key"),
				filepath.Join(home, ".shodan", "api_key"),
			}
			for _, p := range candidates {
				data, err := os.ReadFile(p)
				if err == nil {
					val := strings.TrimSpace(string(data))
					if val != "" {
						k.Shodan = val
						break
					}
				}
			}
		}
	}

	return k
}

// DetectWordlist returns the first existing wordlist path from a priority list.
// If userProvided is non-empty and exists it is returned immediately.
func DetectWordlist(userProvided string) string {
	home, _ := os.UserHomeDir()

	candidates := []string{
		userProvided,
		"/usr/share/seclists/Discovery/DNS/subdomains-top1million-20000.txt",
		"/usr/share/seclists/Discovery/DNS/subdomains-top1million-5000.txt",
		"/usr/share/wordlists/seclists/Discovery/DNS/subdomains-top1million-5000.txt",
	}

	if home != "" {
		candidates = append(candidates,
			filepath.Join(home, "SecLists", "Discovery", "DNS", "subdomains-top1million-5000.txt"),
			filepath.Join(home, "wordlists", "subdomains.txt"),
		)
	}

	for _, p := range candidates {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// LoadDomains populates c.Domains and c.Label from c.Domain or c.DomainFile.
// Lines that are blank or start with '#' are skipped.
func (c *Config) LoadDomains() error {
	if c.Domain == "" && c.DomainFile == "" {
		return fmt.Errorf("provide -d <domain> or -dL <file>")
	}

	if c.DomainFile != "" {
		f, err := os.Open(c.DomainFile)
		if err != nil {
			return fmt.Errorf("open domain file: %w", err)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			c.Domains = append(c.Domains, line)
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("reading domain file: %w", err)
		}
		c.Label = strings.TrimSuffix(filepath.Base(c.DomainFile), ".txt")
	} else {
		c.Domains = []string{c.Domain}
		c.Label = c.Domain
	}

	if len(c.Domains) == 0 {
		return fmt.Errorf("no valid domains found")
	}
	return nil
}

// SetupPaths sets TmpDir and SourcesDir, then creates them on disk.
// OutDir defaults to ./results/<label> when not set.
func (c *Config) SetupPaths() {
	if c.OutDir == "" {
		c.OutDir = filepath.Join("results", c.Label)
	}
	c.TmpDir = filepath.Join(c.OutDir, ".tmp")
	c.SourcesDir = filepath.Join(c.TmpDir, "sources")

	_ = os.MkdirAll(c.OutDir, 0o755)
	_ = os.MkdirAll(c.TmpDir, 0o755)
	_ = os.MkdirAll(c.SourcesDir, 0o755)
}

// DomainListFile returns the path to the domain list file and writes the
// domains to that file, creating it if necessary.
func (c *Config) DomainListFile() string {
	path := filepath.Join(c.TmpDir, "domains.txt")

	f, err := os.Create(path)
	if err != nil {
		return path
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, d := range c.Domains {
		_, _ = fmt.Fprintln(w, d)
	}
	_ = w.Flush()

	return path
}
