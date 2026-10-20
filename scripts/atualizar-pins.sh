#!/usr/bin/env bash
# Confere se há versão nova de cada código de terceiros/pessoal fixado no
# post-install e, se houver, atualiza o pin no lugar. Usado pelo workflow
# .github/workflows/pins.yml (que abre um PR com o resultado), mas também
# roda à mão a partir da raiz do repositório.
#
# Pins conferidos:
#   - internal/modules/desktop.go: pares <nome>Repo/<nome>Commit (último
#     commit da branch principal) e yaruTag/yaruSHA256 (última tag numérica)
#   - GNOME/extensoes.txt: linhas "uuid url commit" (último commit)
#   - internal/modules/apps.go + 42-logitech-unify-permissions.rules: regra
#     UDEV do Solaar (última tag estável)
#
# O resumo das mudanças (em Markdown, para o corpo do PR) vai para o arquivo
# em $PINS_RESUMO (padrão: pins-resumo.md).
set -euo pipefail

DESKTOP="internal/modules/desktop.go"
APPS="internal/modules/apps.go"
EXTENSOES="GNOME/extensoes.txt"
REGRA="internal/modules/42-logitech-unify-permissions.rules"
RESUMO="${PINS_RESUMO:-pins-resumo.md}"

: >"$RESUMO"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

anotar() { echo "- $*" | tee -a "$RESUMO"; }

# URL web do repositório (sem .git), para os links de comparação.
web() { echo "${1%.git}"; }

ultimo_commit() { git ls-remote "$1" HEAD | cut -f1; }

# Última tag que casa com o padrão (ERE), em ordem de versão.
ultima_tag() {
    git ls-remote --tags --refs "$1" | sed 's#.*refs/tags/##' | grep -E "$2" | sort -V | tail -1
}

# ---- Commits fixados em desktop.go ----
while read -r nome atual; do
    repo="$(sed -nE "s/^\s*${nome}Repo\s*=\s*\"([^\"]+)\".*/\1/p" "$DESKTOP")"
    if [[ -z "$repo" ]]; then
        echo "aviso: ${nome}Repo não encontrado em $DESKTOP" >&2
        continue
    fi
    novo="$(ultimo_commit "$repo")"
    if [[ -n "$novo" && "$novo" != "$atual" ]]; then
        sed -i "s/$atual/$novo/" "$DESKTOP"
        anotar "**${nome}**: [\`${atual:0:7}...${novo:0:7}\`]($(web "$repo")/compare/$atual...$novo)"
    fi
done < <(sed -nE 's/^\s*(\w+)Commit\s*=\s*"([0-9a-f]{40})".*/\1 \2/p' "$DESKTOP")

# ---- Commits fixados em extensoes.txt ----
while read -r uuid repo atual; do
    novo="$(ultimo_commit "$repo")"
    if [[ -n "$novo" && "$novo" != "$atual" ]]; then
        sed -i "s/$atual/$novo/" "$EXTENSOES"
        anotar "**${uuid}**: [\`${atual:0:7}...${novo:0:7}\`]($(web "$repo")/compare/$atual...$novo)"
    fi
done < <(grep -E '^[^#-]\S*\s+https://\S+\s+[0-9a-f]{40}\s*$' "$EXTENSOES" || true)

# ---- Yaru (tag + SHA-256 do tarball) ----
yaru_atual="$(sed -nE 's/^\s*yaruTag\s*=\s*"([^"]+)".*/\1/p' "$DESKTOP")"
yaru_novo="$(ultima_tag https://github.com/ubuntu/yaru.git '^[0-9]+\.[0-9]+(\.[0-9]+)?$')"
if [[ -n "$yaru_novo" && "$yaru_novo" != "$yaru_atual" ]]; then
    curl -fsSL --proto '=https' "https://github.com/ubuntu/yaru/archive/refs/tags/$yaru_novo.tar.gz" -o "$TMP/yaru.tar.gz"
    sha="$(sha256sum "$TMP/yaru.tar.gz" | cut -d' ' -f1)"
    sed -i -E "s/^(\s*yaruTag\s*=\s*)\"[^\"]+\"/\1\"$yaru_novo\"/; s/^(\s*yaruSHA256\s*=\s*)\"[0-9a-f]+\"/\1\"$sha\"/" "$DESKTOP"
    anotar "**Yaru**: [\`$yaru_atual...$yaru_novo\`](https://github.com/ubuntu/yaru/compare/$yaru_atual...$yaru_novo)"
fi

# ---- Regra UDEV do Solaar ----
solaar_atual="$(sed -nE 's/^const solaarTag = "([^"]+)".*/\1/p' "$APPS")"
solaar_novo="$(ultima_tag https://github.com/pwr-Solaar/Solaar.git '^[0-9]+\.[0-9]+\.[0-9]+$')"
if [[ -n "$solaar_novo" && "$solaar_novo" != "$solaar_atual" ]]; then
    curl -fsSL --proto '=https' "https://raw.githubusercontent.com/pwr-Solaar/Solaar/$solaar_novo/rules.d-uinput/42-logitech-unify-permissions.rules" -o "$TMP/regra"
    sed -i "s/^const solaarTag = \"$solaar_atual\"/const solaarTag = \"$solaar_novo\"/" "$APPS"
    if cmp -s "$TMP/regra" "$REGRA"; then
        anotar "**Solaar**: $solaar_atual → $solaar_novo (regra UDEV sem mudanças)"
    else
        cp "$TMP/regra" "$REGRA"
        anotar "**Solaar**: $solaar_atual → $solaar_novo — **a regra UDEV mudou** (roda como root: revise o diff com atenção)"
    fi
fi

if [[ ! -s "$RESUMO" ]]; then
    echo "Todos os pins já estão na versão mais recente."
fi
