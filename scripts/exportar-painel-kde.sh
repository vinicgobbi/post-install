#!/usr/bin/env bash
# Exporta os painéis do Plasma desta máquina para GNOME/painel-kde.appletsrc
# e GNOME/painel-kde.plasmashellrc, o modelo que o módulo apps_gnome aplica
# no KDE ("Painel do Plasma"). Copia, sem mudar nada, todas as seções de
# cada painel (os widgets, a bandeja e as configurações deles) e das views
# dos painéis (espessura, flutuante...), inclusive os painéis de monitores
# desconectados agora. Roda à mão a partir da raiz do repositório, na sessão
# do Plasma já configurada:
#
#   scripts/exportar-painel-kde.sh
set -euo pipefail

cfg="${XDG_CONFIG_HOME:-$HOME/.config}"
applets="$cfg/plasma-org.kde.plasma.desktop-appletsrc"
shellrc="$cfg/plasmashellrc"
destino="GNOME"

for f in "$applets" "$shellrc"; do
	[ -f "$f" ] || { echo "não encontrado: $f" >&2; exit 1; }
done

# IDs das contêineres de painel: [Containments][N] com plugin=org.kde.panel.
ids="$(awk '
	/^\[/ { id = ""; if ($0 ~ /^\[Containments\]\[[0-9]+\]$/) { id = $0; gsub(/[^0-9]/, "", id) } next }
	id != "" && $0 == "plugin=org.kde.panel" { print id }
' "$applets")"
[ -n "$ids" ] || { echo "nenhum painel em $applets" >&2; exit 1; }

# extrair imprime as seções de arquivo cujo cabeçalho começa por antes+ID+depois
# para algum dos IDs dos painéis.
extrair() {
	awk -v ids="$ids" -v antes="$2" -v depois="$3" '
		BEGIN { n = split(ids, lista, "\n") }
		/^\[/ {
			dentro = 0
			for (i = 1; i <= n; i++)
				if (index($0, antes lista[i] depois) == 1) { dentro = 1; break }
			if (dentro) { if (impressas++) print ""; print }
			next
		}
		dentro && $0 != "" { print }
	' "$1"
}

cabecalho() {
	cat <<'EOF'
# Painéis do Plasma aplicados pelo módulo "apps_gnome" do post-install no
# KDE. Gerado por scripts/exportar-painel-kde.sh (não edite à mão: configure
# o painel no Plasma e exporte de novo). Os IDs são renumerados na
# aplicação se colidirem com os da máquina.

EOF
}

{ cabecalho; extrair "$applets" "[Containments][" "]"; } >"$destino/painel-kde.appletsrc"
{ cabecalho; extrair "$shellrc" "[PlasmaViews][Panel " "]"; } >"$destino/painel-kde.plasmashellrc"

echo "Painéis exportados ($(echo $ids)) para $destino/painel-kde.appletsrc e $destino/painel-kde.plasmashellrc."
