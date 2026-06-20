#!/usr/bin/env bash
# SysBoard install script
# Usage: curl -fsSL https://raw.githubusercontent.com/hilmyah/SysBoard/master/install.sh | bash
# Or clone the repo and run: bash install.sh

set -euo pipefail

REPO_RAW="https://raw.githubusercontent.com/hilmyah/SysBoard/master"
INSTALL_DIR="/opt/sysboard"
SERVICE_DST="/etc/systemd/system/sysboard.service"

RED='\033[0;31m'; GREEN='\033[0;32m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

info() { echo -e "${CYAN}[*]${NC} $1"; }
ok()   { echo -e "${GREEN}[+]${NC} $1"; }
fail() { echo -e "${RED}[!]${NC} $1"; exit 1; }
hr()   { echo -e "${CYAN}---${NC}"; }

# ── Preflight checks ──────────────────────────────────────────────────────────

[ "$(id -u)" -eq 0 ] || fail "Must be run as root."
command -v go    &>/dev/null || fail "Go not found. Install Go >= 1.21 and re-run."
command -v curl  &>/dev/null || fail "curl not found."
command -v openssl &>/dev/null || fail "openssl not found."

GO_MIN=21
GO_VER=$(go version | grep -oP 'go1\.\K[0-9]+')
[ "$GO_VER" -ge "$GO_MIN" ] 2>/dev/null || fail "Go >= 1.21 required (found go1.${GO_VER})."

# ── Feature selection ─────────────────────────────────────────────────────────

echo ""
echo -e "${BOLD}SysBoard Installer${NC}"
hr
echo "Base install: Overview, Processes, Services"
echo ""
read -rp "Include Containers support (Docker/Podman/nerdctl)? [y/N]: " _CTRS
_CTRS="${_CTRS,,}"

# ── Detect if this is an update ───────────────────────────────────────────────

IS_UPDATE=0
if [ -f "$INSTALL_DIR/sysboard" ]; then
  IS_UPDATE=1
  info "Existing install detected at $INSTALL_DIR -- updating."
fi

# ── Create directory layout ───────────────────────────────────────────────────

info "Preparing $INSTALL_DIR ..."
mkdir -p "$INSTALL_DIR/static" "$INSTALL_DIR/systemd"

# ── Download source files ─────────────────────────────────────────────────────

info "Downloading Go source files..."
for f in main.go middleware.go metrics.go processes.go services.go go.mod; do
  curl -fsSL "$REPO_RAW/$f" -o "$INSTALL_DIR/$f"
done

if [[ "$_CTRS" == "y" ]]; then
  curl -fsSL "$REPO_RAW/containers.go" -o "$INSTALL_DIR/containers.go"
  # Remove stub if it was previously installed without container support.
  rm -f "$INSTALL_DIR/containers_stub.go"
  ok "Containers support included."
else
  curl -fsSL "$REPO_RAW/containers_stub.go" -o "$INSTALL_DIR/containers_stub.go"
  # Remove full implementation if switching away from container support.
  rm -f "$INSTALL_DIR/containers.go"
  info "Containers support skipped."
fi

# ── Download frontend ─────────────────────────────────────────────────────────

info "Downloading frontend..."
for f in index.html style.css app.js; do
  curl -fsSL "$REPO_RAW/static/$f" -o "$INSTALL_DIR/static/$f"
done

# ── Download systemd unit ─────────────────────────────────────────────────────

curl -fsSL "$REPO_RAW/systemd/sysboard.service" -o "$INSTALL_DIR/systemd/sysboard.service"

# ── Environment file ──────────────────────────────────────────────────────────

if [ ! -f "$INSTALL_DIR/.env" ]; then
  curl -fsSL "$REPO_RAW/.env.example" -o "$INSTALL_DIR/.env.example"
  cp "$INSTALL_DIR/.env.example" "$INSTALL_DIR/.env"
  TOKEN=$(openssl rand -hex 32)
  sed -i "s|<YOUR_SYSBOARD_TOKEN>|$TOKEN|" "$INSTALL_DIR/.env"
  chmod 600 "$INSTALL_DIR/.env"
  ok ".env created with auto-generated token."
else
  info ".env already exists -- skipping token generation."
fi

# ── Build ─────────────────────────────────────────────────────────────────────

info "Building binary..."
cd "$INSTALL_DIR"
go build -ldflags="-s -w" -o sysboard .
ok "Binary built: $INSTALL_DIR/sysboard"

# ── Systemd ───────────────────────────────────────────────────────────────────

cp "$INSTALL_DIR/systemd/sysboard.service" "$SERVICE_DST"
systemctl daemon-reload

if [ "$IS_UPDATE" -eq 1 ]; then
  systemctl restart sysboard
  ok "Service restarted."
else
  systemctl enable --now sysboard
  ok "Service enabled and started."
fi

# ── Summary ───────────────────────────────────────────────────────────────────

hr
TOKEN_VAL=$(grep "^SYSBOARD_TOKEN" "$INSTALL_DIR/.env" | cut -d= -f2)
PORT_VAL=$(grep "^SYSBOARD_PORT"  "$INSTALL_DIR/.env" 2>/dev/null | cut -d= -f2)
[ -z "$PORT_VAL" ] && PORT_VAL="8888"
SERVER_IP=$(hostname -I | awk '{print $1}')

if [ "$IS_UPDATE" -eq 1 ]; then
  ok "SysBoard updated successfully."
else
  ok "SysBoard installed successfully."
fi

echo ""
echo -e "  Port:   ${CYAN}${PORT_VAL}${NC}"
echo -e "  Token:  ${CYAN}${TOKEN_VAL}${NC}"
echo -e "  Access: ${CYAN}http://${SERVER_IP}:${PORT_VAL}${NC}"
echo ""
echo "  To view logs:  journalctl -u sysboard -f"
echo "  To reconfigure: nano $INSTALL_DIR/.env && systemctl restart sysboard"
echo ""
