#!/usr/bin/env bash
# =========================================================
# esey-ssh-dev — Installer
# Author: Sumit
# =========================================================

set -Eeuo pipefail
IFS=$'\n\t'
trap 'rc=$?; echo -e "${RED}❌ Error on line $LINENO (exit $rc)${NC}"; exit $rc' ERR

# ---------------- Colors ----------------

RED="\e[31m"
GREEN="\e[32m"
YELLOW="\e[33m"
BLUE="\e[34m"
CYAN="\e[36m"
NC="\e[0m"

log() { echo -e "${CYAN}➜${NC} $*"; }
ok() { echo -e "${GREEN}✔${NC} $*"; }
warn() { echo -e "${YELLOW}⚠${NC} $*"; }
err() { echo -e "${RED}✖${NC} $*"; }

# ---------------- Config ----------------

PY_MIN_MAJOR=3
PY_MIN_MINOR=8
GO_MIN_MAJOR=1
GO_MIN_MINOR=20

DRY_RUN=false
AUTO_YES=false
AUTO_BUILD=false

for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=true ;;
    -y|--yes) AUTO_YES=true ;;
    --build) AUTO_BUILD=true ;;
  esac
done

run() {
  log "$*"
  [[ "$DRY_RUN" == true ]] || "$@"
}

confirm() {
  [[ "$AUTO_YES" == true ]] && return 0
  read -rp "Continue? [y/N]: " ans
  [[ "${ans,,}" == "y" ]]
}

# ---------------- Root Detection ----------------

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_FILE="$PROJECT_ROOT/install.log"

exec > >(tee -a "$LOG_FILE") 2>&1

echo -e "${BLUE}🔧 esey-ssh-dev Installer ${NC}"
echo "Project Root: $PROJECT_ROOT"
echo "----------------------------------------"

# ---------------- Detect OS ----------------

OS=""
PM=""
SUDO=""

if [[ -n "${TERMUX_VERSION:-}" ]]; then
  OS="termux"; PM="pkg"
elif [[ "$OSTYPE" == "darwin"* ]]; then
  OS="macos"; PM="brew"
elif command -v apt >/dev/null; then
  OS="debian"; PM="apt"
elif command -v dnf >/dev/null; then
  OS="fedora"; PM="dnf"
elif command -v pacman >/dev/null; then
  OS="arch"; PM="pacman"
elif command -v apk >/dev/null; then
  OS="alpine"; PM="apk"
else
  err "Unsupported OS"
  exit 1
fi

if [[ "$EUID" -ne 0 ]] && command -v sudo >/dev/null; then
  SUDO="sudo"
fi

echo "OS: $OS | Package Manager: $PM"
echo "----------------------------------------"

# ---------------- Version Checks ----------------

check_python() {
  command -v python3 >/dev/null || return 1
  python3 - <<EOF
import sys
sys.exit(0 if sys.version_info >= ($PY_MIN_MAJOR,$PY_MIN_MINOR) else 1)
EOF
}

check_go() {
  command -v go >/dev/null || return 1
  ver=$(go version | awk '{print $3}' | sed 's/go//')
  IFS=. read -r major minor _ <<< "$ver"
  (( major > GO_MIN_MAJOR || (major == GO_MIN_MAJOR && minor >= GO_MIN_MINOR) ))
}

echo "Python: $(python3 --version 2>/dev/null || echo missing)"
echo "Go    : $(go version 2>/dev/null || echo missing)"
echo "----------------------------------------"

# ---------------- Package Install ----------------

install_pkgs() {
  case "$PM" in
    apt) run $SUDO apt update -y && run $SUDO apt install -y "$@" ;;
    dnf) run $SUDO dnf install -y "$@" ;;
    pacman) run $SUDO pacman -Sy --noconfirm "$@" ;;
    apk) run $SUDO apk add "$@" ;;
    pkg) run pkg install -y "$@" ;;
    brew) run brew install "$@" ;;
  esac
}

case "$OS" in
  debian) CORE=(golang python3 python3-venv python3-pip jq openssh-client) ;;
  fedora) CORE=(golang python3 python3-pip jq openssh-clients) ;;
  arch) CORE=(go python python-pip jq openssh) ;;
  alpine) CORE=(go python3 py3-pip py3-virtualenv jq openssh) ;;
  termux) CORE=(golang python jq openssh) ;;
  macos) CORE=(go python jq) ;;
esac

echo "Checking dependencies..."

NEED=false

check_python || { warn "Python >=3.8 missing"; NEED=true; }
check_go || { warn "Go >=1.20 missing"; NEED=true; }

for cmd in jq ssh; do
  command -v "$cmd" >/dev/null || { warn "$cmd missing"; NEED=true; }
done

if [[ "$NEED" == true ]]; then
  warn "Missing core dependencies."
  confirm || exit 1
  install_pkgs "${CORE[@]}"
else
  ok "Core dependencies OK"
fi

# ---------------- Python Venv ----------------

VENV="$PROJECT_ROOT/.venv"

if [[ ! -d "$VENV" ]]; then
  log "Creating virtual environment..."
  [[ "$DRY_RUN" == true ]] || python3 -m venv "$VENV"
fi

if [[ -x "$VENV/bin/python" ]]; then
  run "$VENV/bin/python" -m pip install --upgrade pip setuptools wheel pyinstaller
  ok "Python build tools ready"
else
  err "Venv setup failed"
  exit 1
fi

# ---------------- Auto Build (Optional) ----------------

if [[ "$AUTO_BUILD" == true ]]; then
  if [[ -f "$PROJECT_ROOT/app-build-install" ]]; then
    log "Triggering build-install..."
    run bash "$PROJECT_ROOT/app-build-install"
  else
    warn "app-build-install not found"
  fi
fi

# ---------------- Summary ----------------

echo ""
echo "----------------------------------------"
echo "Log file: $LOG_FILE"
