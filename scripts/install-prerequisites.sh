#!/usr/bin/env bash
set -euo pipefail

# Installs local prerequisites for AgenticSDLC on Debian/Ubuntu.
# For other OSes, use the Docker-based recommended method in README.

if [[ "${EUID}" -ne 0 ]]; then
  SUDO="sudo"
else
  SUDO=""
fi

command -v apt-get >/dev/null 2>&1 || {
  echo "This installer currently supports Debian/Ubuntu only (apt-get not found)."
  exit 1
}

echo "[1/5] Updating package index"
$SUDO apt-get update

echo "[2/5] Installing base utilities"
$SUDO apt-get install -y ca-certificates curl git gnupg lsb-release make

echo "[3/5] Installing Docker"
if ! command -v docker >/dev/null 2>&1; then
  $SUDO install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg | $SUDO gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  $SUDO chmod a+r /etc/apt/keyrings/docker.gpg

  echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
    $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
    $SUDO tee /etc/apt/sources.list.d/docker.list >/dev/null

  $SUDO apt-get update
  $SUDO apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
else
  echo "Docker already installed; skipping"
fi

echo "[4/5] Installing Go"
if ! command -v go >/dev/null 2>&1; then
  GO_VERSION="1.24.3"
  curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tgz
  $SUDO rm -rf /usr/local/go
  $SUDO tar -C /usr/local -xzf /tmp/go.tgz
  rm -f /tmp/go.tgz
  if ! grep -q '/usr/local/go/bin' /etc/profile; then
    echo 'export PATH=$PATH:/usr/local/go/bin' | $SUDO tee -a /etc/profile >/dev/null
  fi
else
  echo "Go already installed; skipping"
fi

echo "[5/5] Final checks"
set +e
docker --version
go version
git --version
set -e

echo "Done. Re-open your shell session before running go commands if Go was newly installed."
