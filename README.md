# subwofer

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
