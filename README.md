# Subwofer

Subdomain enumeration pipeline. Runs passive sources, DNS bruteforce, permutations, resolving, and HTTP probing in sequence.

## Install

```bash
bash install.sh
```

Then build:

```bash
go build -o subwofer .
```

## Usage

```bash
# single domain
./subwofer -d example.com

# list of domains
./subwofer -dL targets.txt

# skip bruteforce and permutation (passive only)
./subwofer -d example.com -B -P

# with screenshots
./subwofer -d example.com -s

# virtual host discovery
./subwofer -d example.com --vhost

# custom output dir and threads
./subwofer -d example.com -o ./out -t 200
```

## Flags

| Flag | Description |
|------|-------------|
| `-d` | Single target domain |
| `-dL` | File with one domain per line |
| `-o` | Output directory (default: `results/<domain>`) |
| `-t` | Threads (default: 100) |
| `-w` | Custom wordlist for bruteforce |
| `-s` | Take screenshots with gowitness |
| `--vhost` | Virtual host discovery with ffuf |
| `--vhost-wordlist` | Custom wordlist for vhost fuzzing |
| `-B` | Skip bruteforce phase |
| `-P` | Skip permutation phase |
| `-H` | Skip HTTP probing |
| `--perm-full` | Run gotator and altdns in addition to alterx |
| `-perm-max` | Cap permutation base size (default: 300, 0 = unlimited) |
| `-v` | Verbose output |

## Tools

### Passive Enumeration

| Tool | Source | API Key |
|------|--------|---------|
| subfinder | projectdiscovery | `PDCP_KEY` (optional, enables `-all`) |
| amass | owasp-amass | — |
| findomain | Findomain | — |
| assetfinder | tomnomnom | — |
| chaos | projectdiscovery | `PDCP_KEY` (required) |
| github-subdomains | gwen001 | `GITHUB_TOKEN` (required) |
| crtsh | crt.sh API (HTTP) | — |
| waybackurls | tomnomnom | — |
| gau | lc | — |
| haktrails | hakluke | `PDCP_KEY` (required) |
| shodan | shodan CLI | `SHODAN_KEY` (required) |
| bevigil-cli | bevigil-osint | `BEVIGIL_KEY` (required) |

### Bruteforce

| Tool | Notes |
|------|-------|
| puredns | preferred |
| shuffledns | fallback if puredns not installed |

### Permutation & Alteration

| Tool | Notes |
|------|-------|
| alterx | default, pattern-based |
| gotator | `--perm-full` only |
| altdns | `--perm-full` only |
| dnsx | used to resolve permutation candidates (optional) |

### Resolution, Probing & Extras

| Tool | Phase |
|------|-------|
| dnsx | DNS resolution |
| httpx | HTTP probing |
| gowitness | Screenshots (`-s`) |
| ffuf | Virtual host discovery (`--vhost`) |

## API Keys

Set via environment variables. All optional.

```bash
export GITHUB_TOKEN=...
export SECURITYTRAILS_KEY=...
export SHODAN_KEY=...
export BEVIGIL_KEY=...
export PDCP_KEY=...
```

## Output

Results saved in `results/<domain>/`:

- `subdomains.txt` — all unique subdomains
- `subdomains_resolved.txt` — resolved only
- `live_urls.txt` — live HTTP/HTTPS URLs
- `screenshots/` — screenshots (if `-s` used)
- `vhosts_found.txt` — virtual hosts (if `--vhost` used)
