#!/bin/sh
# Sincroniza os temas Yaru de /usr/share/themes com uma cópia fora de /usr,
# que os apps Flatpak conseguem enxergar (o Flatpak não expõe /usr do host).
# Instalado pelo post-install e chamado pelo hook do APT
# (/etc/apt/apt.conf.d/99yaru-flatpak) depois de cada execução do dpkg: só
# copia de novo quando a versão do yaru-theme-gtk muda (ou com --force).
set -eu

origem="${YARU_FLATPAK_ORIGEM:-/usr/share/themes}"
base="${YARU_FLATPAK_BASE:-/var/lib/yaru-flatpak}"
destino="$base/themes"
marca="$base/versao"

versao="$(dpkg-query -W -f='${Version}' yaru-theme-gtk 2>/dev/null || true)"
if [ "${1:-}" != "--force" ] && [ -d "$destino" ] && [ -f "$marca" ] && [ "$(cat "$marca")" = "$versao" ]; then
    exit 0
fi

mkdir -p "$base"
novo="$(mktemp -d "$base/.themes.XXXXXX")"
trap 'rm -rf "$novo"' EXIT
for tema in "$origem"/Yaru*; do
    [ -d "$tema" ] || continue
    # -L: os temas do Ubuntu têm links para arquivos de outros temas (e de
    # fora de /usr/share/themes), que não existiriam dentro do sandbox.
    cp -rL "$tema" "$novo/"
done
chmod 755 "$novo"

# Troca a cópia de uma vez, para um app aberto no meio não ver metade.
rm -rf "$destino.antigo"
if [ -d "$destino" ]; then
    mv "$destino" "$destino.antigo"
fi
mv "$novo" "$destino"
rm -rf "$destino.antigo"
printf '%s\n' "$versao" >"$marca"
