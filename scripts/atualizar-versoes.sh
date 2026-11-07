#!/usr/bin/env bash
# Regrava internal/sysinfo/versoes.go com as versões suportadas, a partir
# das fontes oficiais. Usado pelo workflow .github/workflows/versoes.yml (que
# abre um PR com o resultado), mas também roda à mão a partir da raiz do
# repositório. Requer curl e jq.
#
#   - Ubuntu: LTS já lançadas e com suporte padrão (coluna "eol" do
#     distro-info-data, a mesma fonte do "ubuntu-distro-info --supported";
#     o ESM pago não conta).
#   - Fedora: versões "current" no Bodhi com fim de vida ainda não alcançado.
#   - Repositório da Microsoft (msodbcsql/mssql-tools): no Ubuntu, a própria
#     LTS se já houver pacote publicado, senão a LTS anterior; no Fedora, o
#     RHEL mais novo publicado.
#
# O resumo das mudanças (em Markdown, para o corpo do PR) vai para o arquivo
# em $VERSOES_RESUMO (padrão: versoes-resumo.md). $HOJE (AAAA-MM-DD) troca a
# data de referência, para testar.
set -euo pipefail

ARQ="internal/sysinfo/versoes.go"
RESUMO="${VERSOES_RESUMO:-versoes-resumo.md}"
HOJE="${HOJE:-$(date -u +%F)}"
UBUNTU_CSV="https://salsa.debian.org/debian/distro-info-data/-/raw/main/ubuntu.csv"
BODHI="https://bodhi.fedoraproject.org/releases/?exclude_archived=1&rows_per_page=100"
MS="https://packages.microsoft.com/config"

: >"$RESUMO"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# existe confere uma URL sem mostrar o erro (404 é resposta esperada aqui).
existe() { curl -fsL -o /dev/null --head --retry 2 "$1"; }

# ---- Ubuntu ----
# Colunas: version,codename,series,created,release,eol,eol-server,eol-esm,...
curl -fsSL --retry 2 "$UBUNTU_CSV" >"$TMP/ubuntu.csv"
awk -F, -v hoje="$HOJE" '
    NR > 1 && $1 ~ / LTS$/ && $5 != "" && $5 <= hoje && $6 > hoje {
        v = $1; sub(/ LTS$/, "", v); print v, $3
    }' "$TMP/ubuntu.csv" | sort -V >"$TMP/ubuntu"

ubuntu_linhas=""
ms_anterior=""
while read -r versao codinome; do
    ms="$versao"
    if ! existe "$MS/ubuntu/$versao/packages-microsoft-prod.deb"; then
        if [[ -n "$ms_anterior" ]]; then
            ms="$ms_anterior"
        else
            echo "aviso: sem repositório da Microsoft para o Ubuntu $versao" >&2
        fi
    fi
    ms_anterior="$ms"
    ubuntu_linhas+=$'\t'"{Versao: \"$versao\", Codinome: \"$codinome\", MSRepo: \"$ms\"},"$'\n'
done <"$TMP/ubuntu"

# ---- Fedora ----
curl -fsSL --retry 2 "$BODHI" |
    jq -r --arg hoje "$HOJE" '.releases[]
        | select(.id_prefix == "FEDORA" and .state == "current" and (.eol == null or .eol > $hoje))
        | .version' | sort -n -u >"$TMP/fedora"

rhel=""
for r in $(seq 9 20); do
    if existe "$MS/rhel/$r/prod.repo"; then
        rhel="$r"
    fi
done
if [[ -z "$rhel" ]]; then
    echo "erro: nenhum repositório da Microsoft para RHEL encontrado" >&2
    exit 1
fi

fedora_linhas=""
while read -r versao; do
    fedora_linhas+=$'\t'"{Versao: \"$versao\", MSRepo: \"$rhel\"},"$'\n'
done <"$TMP/fedora"

# Uma resposta vazia (fonte fora do ar, formato mudado) não pode virar um PR
# que tira o suporte a tudo.
if [[ -z "$ubuntu_linhas" || -z "$fedora_linhas" ]]; then
    echo "erro: lista de versões vazia (Ubuntu: $(wc -l <"$TMP/ubuntu"), Fedora: $(wc -l <"$TMP/fedora")); nada alterado" >&2
    exit 1
fi

cat >"$TMP/versoes.go" <<EOF
// Versões suportadas — mantido pelo scripts/atualizar-versoes.sh, que o
// workflow versoes.yml roda toda semana, abrindo um PR quando algo muda.
// Pode ser editado à mão, mas o script regrava o arquivo inteiro.

package sysinfo

// ubuntuSuportadas são as LTS do Ubuntu com suporte padrão (sem contar o
// ESM), da mais antiga para a mais nova. MSRepo é a versão usada nas URLs do
// repositório da Microsoft (msodbcsql/mssql-tools): a própria, ou a da LTS
// anterior enquanto a Microsoft não publica a nova.
var ubuntuSuportadas = []versaoUbuntu{
${ubuntu_linhas}}

// fedoraSuportadas são as versões do Fedora em manutenção, da mais antiga
// para a mais nova. MSRepo é a versão do RHEL usada nas URLs do repositório
// da Microsoft (que não publica pacotes para o Fedora).
var fedoraSuportadas = []versaoFedora{
${fedora_linhas}}
EOF

if cmp -s "$TMP/versoes.go" "$ARQ"; then
    echo "Versões suportadas sem mudança."
    exit 0
fi

# Resumo: as entradas que saem e as que entram.
entradas() { grep -oE '\{Versao: [^}]*\}' "$1" | sed -E 's/[{}]//g' || true; }
# (o diff sai com 1 quando há diferença, o que é o caso aqui)
{ diff <(entradas "$ARQ") <(entradas "$TMP/versoes.go") || true; } | sed -nE \
    -e 's/^< (.*)/- **sai:** `\1`/p' \
    -e 's/^> (.*)/- **entra:** `\1`/p' | tee -a "$RESUMO"
cp "$TMP/versoes.go" "$ARQ"
