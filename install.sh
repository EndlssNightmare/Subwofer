#!/usr/bin/env bash
set -e

# Detect distro
if command -v apt-get &>/dev/null; then
    DISTRO="debian"
elif command -v pacman &>/dev/null; then
    DISTRO="arch"
else
    echo "Unsupported distro. Install Go and the tools manually."
    exit 1
fi

install_go() {
    if command -v go &>/dev/null; then
        echo "[*] Go already installed: $(go version)"
        return
    fi

    echo "[*] Installing Go..."
    if [ "$DISTRO" = "debian" ]; then
        sudo apt-get update -qq
        sudo apt-get install -y golang-go
    elif [ "$DISTRO" = "arch" ]; then
        sudo pacman -Sy --noconfirm go
    fi
}

install_python_tools() {
    echo "[*] Installing Python tools..."
    if ! command -v pip3 &>/dev/null; then
        if [ "$DISTRO" = "debian" ]; then
            sudo apt-get install -y python3-pip
        elif [ "$DISTRO" = "arch" ]; then
            sudo pacman -Sy --noconfirm python-pip
        fi
    fi
    pip3 install --user shodan bevigil-osint 2>/dev/null || true
}

install_go_tool() {
    local name="$1"
    local pkg="$2"
    if command -v "$name" &>/dev/null; then
        echo "[~] $name already installed, skipping"
        return
    fi
    echo "[*] Installing $name..."
    go install "$pkg" 2>/dev/null && echo "[+] $name installed" || echo "[!] Failed to install $name"
}

install_go
install_python_tools

export PATH="$PATH:$(go env GOPATH)/bin"

# Core
install_go_tool subfinder   "github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest"
install_go_tool dnsx        "github.com/projectdiscovery/dnsx/cmd/dnsx@latest"
install_go_tool httpx       "github.com/projectdiscovery/httpx/cmd/httpx@latest"
install_go_tool puredns     "github.com/d3mondev/puredns/v2@latest"
install_go_tool alterx      "github.com/projectdiscovery/alterx/cmd/alterx@latest"

# Passive sources
install_go_tool assetfinder     "github.com/tomnomnom/assetfinder@latest"
install_go_tool waybackurls     "github.com/tomnomnom/waybackurls@latest"
install_go_tool gau             "github.com/lc/gau/v2/cmd/gau@latest"
install_go_tool amass           "github.com/owasp-amass/amass/v4/...@master"
install_go_tool chaos           "github.com/projectdiscovery/chaos-client/cmd/chaos@latest"
install_go_tool haktrails       "github.com/hakluke/haktrails@latest"
install_go_tool github-subdomains "github.com/gwen001/github-subdomains@latest"
install_go_tool findomain       "github.com/Findomain/Findomain@latest"

# Permutation (optional, used with --perm-full)
install_go_tool gotator     "github.com/Josue87/gotator@latest"

# Optional extras
install_go_tool gowitness   "github.com/sensepost/gowitness@latest"
install_go_tool ffuf        "github.com/ffuf/ffuf/v2@latest"
install_go_tool shuffledns  "github.com/projectdiscovery/shuffledns/cmd/shuffledns@latest"

echo ""
echo "[+] Done. Build subwofer with: go build -o subwofer ."
echo "[!] Make sure $(go env GOPATH)/bin is in your PATH."
