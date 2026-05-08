package normalize

import (
	"bufio"
	"os"
	"regexp"
	"sort"
	"strings"
)

// BuildDomainRegex builds a compiled regex that matches any valid subdomain
// of any of the supplied root domains.
// Pattern example: (?i)([a-zA-Z0-9._-]+\.example\.com|[a-zA-Z0-9._-]+\.other\.org)
func BuildDomainRegex(domains []string) *regexp.Regexp {
	if len(domains) == 0 {
		return nil
	}

	var parts []string
	for _, d := range domains {
		parts = append(parts, `[a-zA-Z0-9._-]+\.`+regexp.QuoteMeta(d))
	}

	pattern := `(?i)(` + strings.Join(parts, "|") + `)`
	return regexp.MustCompile(pattern)
}

// NormalizeRaw reads rawPath, extracts valid subdomains using the domain regex,
// lowercases them, strips leading dots, deduplicates, sorts and writes to
// outPath. Returns the number of unique subdomains written.
func NormalizeRaw(rawPath string, domains []string, outPath string) (int, error) {
	re := BuildDomainRegex(domains)
	if re == nil {
		return 0, nil
	}

	f, err := os.Open(rawPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(f)
	// Increase buffer for potentially very long lines from some tools.
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindAllString(line, -1)
		for _, m := range matches {
			m = strings.ToLower(m)
			m = strings.TrimLeft(m, ".")
			if m != "" {
				seen[m] = struct{}{}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}

	unique := make([]string, 0, len(seen))
	for s := range seen {
		unique = append(unique, s)
	}
	sort.Strings(unique)

	out, err := os.Create(outPath)
	if err != nil {
		return 0, err
	}
	defer out.Close()

	w := bufio.NewWriter(out)
	for _, s := range unique {
		if _, err := w.WriteString(s + "\n"); err != nil {
			return 0, err
		}
	}
	return len(unique), w.Flush()
}

// CountLines returns the number of newline-terminated lines in path.
// Returns 0 on any error.
func CountLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)
	for scanner.Scan() {
		if scanner.Text() != "" {
			count++
		}
	}
	return count
}
