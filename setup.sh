#!/usr/bin/env bash
# Bootstrap do post-install: baixa o binário da release no GitHub (verificando
# o SHA-256) e executa como root, apontando OVPN/ e logs/ para esta pasta.
#
#   sudo ./setup.sh                          # última release
#   POST_INSTALL_VERSION=v1.2.0 ./setup.sh   # versão específica
#   ./setup.sh --dry-run                     # flags são repassadas ao binário
#
# Se existir um ./post-install compilado localmente (go build -o post-install
# ./cmd/post-install), ele é usado no lugar do download.
set -euo pipefail

REPO="vinicgobbi/post-install"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ "$EUID" -ne 0 ]]; then
    exec sudo POST_INSTALL_VERSION="${POST_INSTALL_VERSION:-}" "$0" "$@"
fi

# Só há build para amd64.
if [[ "$(uname -m)" != "x86_64" ]]; then
    echo "[-] Arquitetura não suportada: $(uname -m) (só amd64/x86_64)." >&2
    exit 1
fi
ARCH="amd64"

baixar() {
    if command -v curl &>/dev/null; then
        curl -fL --retry 3 --progress-bar -o "$2" "$1"
    else
        wget -q --show-progress -O "$2" "$1"
    fi
}

BIN="$SCRIPT_DIR/post-install"
if [[ ! -x "$BIN" ]]; then
    ASSET="post-install-linux-$ARCH"
    if [[ -n "${POST_INSTALL_VERSION:-}" ]]; then
        URL="https://github.com/$REPO/releases/download/$POST_INSTALL_VERSION/$ASSET"
    else
        URL="https://github.com/$REPO/releases/latest/download/$ASSET"
    fi

    TMP_DIR="$(mktemp -d)"
    echo "[*] Baixando $URL"
    baixar "$URL" "$TMP_DIR/$ASSET"
    baixar "$URL.sha256" "$TMP_DIR/$ASSET.sha256"
    (cd "$TMP_DIR" && sha256sum --quiet -c "$ASSET.sha256") || {
        echo "[-] Checksum SHA-256 não confere; abortando." >&2
        exit 1
    }
    chmod +x "$TMP_DIR/$ASSET"
    BIN="$TMP_DIR/$ASSET"
fi

exec "$BIN" --ovpn-dir "$SCRIPT_DIR/OVPN" --log-dir "$SCRIPT_DIR/logs" "$@"
