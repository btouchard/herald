#!/bin/sh
set -eu

REPO="btouchard/herald"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY="herald"
CONFIG_DIR="${HOME}/.config/herald"
CONFIG_FILE="${CONFIG_DIR}/herald.yaml"

main() {
    os="$(detect_os)"
    arch="$(detect_arch)"
    version="$(fetch_latest_version)"

    printf "Installing %s %s (%s/%s)...\n" "$BINARY" "$version" "$os" "$arch"

    ver="${version#v}"
    archive="${BINARY}_${ver}_${os}_${arch}.tar.gz"
    url="https://github.com/${REPO}/releases/download/${version}/${archive}"
    checksums_url="https://github.com/${REPO}/releases/download/${version}/checksums.txt"

    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT

    printf "Downloading %s...\n" "$archive"
    download "$url" "$tmpdir/$archive"
    download "$checksums_url" "$tmpdir/checksums.txt"

    printf "Verifying checksum...\n"
    verify_checksum "$tmpdir" "$archive"

    printf "Extracting...\n"
    tar -xzf "$tmpdir/$archive" -C "$tmpdir"

    printf "Installing to %s...\n" "$INSTALL_DIR"
    install_binary "$tmpdir/$BINARY" "$INSTALL_DIR/$BINARY"

    printf "\n✓ %s %s installed at %s/%s\n\n" "$BINARY" "$version" "$INSTALL_DIR" "$BINARY"

    if ask_yn "Run setup wizard" "y"; then
        setup_wizard
    else
        printf "\nTo configure manually, see: https://github.com/%s/blob/main/configs/herald.example.yaml\n" "$REPO"
    fi
}

detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        *)       printf "Unsupported OS: %s\n" "$(uname -s)" >&2; exit 1 ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *)             printf "Unsupported architecture: %s\n" "$(uname -m)" >&2; exit 1 ;;
    esac
}

fetch_latest_version() {
    url="https://api.github.com/repos/${REPO}/releases/latest"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" | parse_version
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "$url" | parse_version
    else
        printf "curl or wget required\n" >&2; exit 1
    fi
}

parse_version() {
    sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1
}

download() {
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL -o "$2" "$1"
    else
        wget -qO "$2" "$1"
    fi
}

verify_checksum() {
    dir="$1"
    file="$2"
    expected="$(grep "$file" "$dir/checksums.txt" | awk '{print $1}')"
    if [ -z "$expected" ]; then
        printf "Checksum not found for %s\n" "$file" >&2; exit 1
    fi
    actual="$(sha256sum "$dir/$file" 2>/dev/null || shasum -a 256 "$dir/$file" 2>/dev/null)"
    actual="$(echo "$actual" | awk '{print $1}')"
    if [ "$expected" != "$actual" ]; then
        printf "Checksum mismatch: expected %s, got %s\n" "$expected" "$actual" >&2; exit 1
    fi
}

install_binary() {
    src="$1"
    dst="$2"
    if [ -w "$(dirname "$dst")" ]; then
        mv "$src" "$dst"
        chmod +x "$dst"
    else
        printf "Need elevated permissions for %s\n" "$(dirname "$dst")"
        sudo mv "$src" "$dst"
        sudo chmod +x "$dst"
    fi
}

ask() {
    prompt="$1"
    default="$2"
    printf "%s [%s]: " "$prompt" "$default" >&2
    read -r answer
    echo "${answer:-$default}"
}

ask_yn() {
    prompt="$1"
    default="$2"
    answer="$(ask "$prompt (y/n)" "$default")"
    case "$answer" in
        [Yy]*) return 0 ;;
        *) return 1 ;;
    esac
}

setup_wizard() {
    printf "\n=== Herald Setup Wizard ===\n\n"

    if [ -f "$CONFIG_FILE" ]; then
        if ! ask_yn "Config already exists at $CONFIG_FILE. Overwrite" "n"; then
            printf "Setup cancelled.\n"
            return
        fi
    fi

    port="$(ask "Port" "8420")"

    printf "\nHow will Claude Chat reach Herald?\n"
    printf "  1) ngrok tunnel (easiest, recommended)\n"
    printf "  2) External domain (you handle TLS with Traefik/Caddy/nginx)\n"
    printf "  3) Local only (development)\n"
    exposure="$(ask "Choice" "1")"

    tunnel_enabled="false"
    tunnel_provider="ngrok"
    tunnel_authtoken=""
    tunnel_domain=""
    public_url=""

    case "$exposure" in
        1)
            tunnel_enabled="true"
            tunnel_authtoken="$(ask "ngrok auth token (from https://dashboard.ngrok.com)" "")"
            while [ -z "$tunnel_authtoken" ]; do
                printf "  Error: ngrok auth token is required. Get yours at https://dashboard.ngrok.com\n" >&2
                tunnel_authtoken="$(ask "ngrok auth token" "")"
            done
            tunnel_domain="$(ask "Fixed ngrok domain (leave empty for random URL)" "")"
            public_url=""
            ;;
        2)
            public_url="$(ask "Public URL (e.g. https://herald.example.com)" "")"
            while ! echo "$public_url" | grep -q '^https://'; do
                printf "Error: Public URL must start with https://\n" >&2
                public_url="$(ask "Public URL (e.g. https://herald.example.com)" "")"
            done
            ;;
        3)
            public_url="http://127.0.0.1:${port}"
            ;;
    esac

    projects=""
    first_project="true"
    if ask_yn "Add a project" "y"; then
        while true; do
            project_name="$(ask "Project name (e.g. my-api)" "")"
            if [ -z "$project_name" ]; then
                printf "Error: Project name cannot be empty\n" >&2
                continue
            fi
            project_path="$(ask "Project path (absolute)" "")"
            if [ -n "$project_path" ] && [ ! -d "$project_path" ]; then
                printf "Warning: Path does not exist (yet): %s\n" "$project_path" >&2
            fi

            if [ "$first_project" = "true" ]; then
                default_flag="true"
                first_project="false"
            else
                default_flag="false"
            fi

            projects="${projects}
  ${project_name}:
    path: \"${project_path}\"
    description: \"${project_name}\"
    default: ${default_flag}
    allowed_tools:
      - \"Read\"
      - \"Write\"
      - \"Edit\"
      - \"Bash(git *)\"
    max_concurrent_tasks: 1
    git:
      auto_branch: true
      auto_stash: false
      auto_commit: false
      branch_prefix: \"herald/\""

            if ! ask_yn "Add another project" "n"; then
                break
            fi
        done
    fi

    mkdir -p "$CONFIG_DIR"
    chmod 700 "$CONFIG_DIR"

    cat > "$CONFIG_FILE" <<EOF
# Herald Configuration
# Generated by install wizard

server:
  host: "127.0.0.1"
  port: ${port}${public_url:+
  public_url: "${public_url}"}
  log_level: "info"

auth:
  client_id: "herald-claude-chat"
  access_token_ttl: 1h
  refresh_token_ttl: 720h
  redirect_uris:
    - "https://claude.ai/oauth/callback"
    - "https://claude.ai/api/oauth/callback"

database:
  path: "~/.config/herald/herald.db"
  retention_days: 90

execution:
  claude_path: "claude"
  model: "claude-sonnet-4-5-20250929"
  default_timeout: 30m
  max_timeout: 2h
  work_dir: "~/.config/herald/work"
  max_concurrent: 3
  max_prompt_size: 102400
  max_output_size: 1048576
  env:
    CLAUDE_CODE_ENTRYPOINT: "herald"
    CLAUDE_CODE_DISABLE_AUTO_UPDATE: "1"
${projects:+
projects:${projects}}

rate_limit:
  requests_per_minute: 60
  burst: 10
EOF

    if [ "$tunnel_enabled" = "true" ]; then
        cat >> "$CONFIG_FILE" <<EOF

tunnel:
  enabled: ${tunnel_enabled}
  provider: "${tunnel_provider}"${tunnel_authtoken:+
  authtoken: "${tunnel_authtoken}"}${tunnel_domain:+
  domain: "${tunnel_domain}"}
EOF
    fi

    printf "\n  ✓ Config written to %s\n" "$CONFIG_FILE"
    printf "  ✓ Secret will be auto-generated on first run\n\n"
    printf "Start Herald:\n"
    printf "  herald serve\n\n"
    printf "Then add Custom Connector in Claude Chat:\n"
    if [ "$exposure" = "1" ]; then
        printf "  URL: <shown at startup for ngrok>/mcp\n"
    else
        printf "  URL: %s/mcp\n" "$public_url"
    fi
    printf "  Client ID: herald-claude-chat\n"
    printf "  Client Secret: shown at startup\n\n"
}

main
