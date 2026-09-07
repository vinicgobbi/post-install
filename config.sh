#!/usr/bin/env bash
# ==========================================
# Configuração compartilhada
# ==========================================

# Detecta o ambiente de desktop atual (kde | gtk). O script roda via sudo,
# que por padrão limpa XDG_CURRENT_DESKTOP (env_reset), então essa variável
# quase nunca chega até aqui — por isso também checamos se o plasmashell está
# instalado no sistema, o que não depende do ambiente herdado do sudo.
# Outros módulos leem $DESKTOP_ENV para pular/adaptar passos específicos de
# GNOME/GTK quando rodando em Plasma.
DESKTOP_ENV="gtk"
if [[ "$XDG_CURRENT_DESKTOP" == *"KDE"* ]] || comando_existe plasmashell; then
    DESKTOP_ENV="kde"
fi

if [[ "$DESKTOP_ENV" == "kde" ]]; then
  # Lista otimizada para o Plasma (com equivalentes em QT + seus apps mantidos)
  FLATPAKS=(
    # Seus apps mantidos por escolha:
    com.spotify.Client io.ente.auth io.github.flattool.Warehouse
    md.obsidian.Obsidian org.libreoffice.LibreOffice
    org.onlyoffice.desktopeditors io.dbeaver.DBeaverCommunity

    # Já são nativos em Qt:
    com.mikrotik.WinBox com.obsproject.Studio org.qbittorrent.qBittorrent
    org.telegram.desktop org.videolan.VLC com.rtosta.zapzap

    # Equivalentes em Qt:
    org.kde.umbrello       # No lugar de Gaphor
    org.kde.krdc           # No lugar de Remmina

    # Sem Flatpak no Flathub (org.kde.krusader não existe); mantido:
    org.filezillaproject.Filezilla
  )
else
  # Mantém exatamente a sua lista original (GTK / Padrão)
  FLATPAKS=(
    com.getpostman.Postman com.github.tchx84.Flatseal
    com.mattjakeman.ExtensionManager 
    com.obsproject.Studio com.spotify.Client dev.vencord.Vesktop
    io.dbeaver.DBeaverCommunity io.ente.auth io.github.flattool.Ignition io.github.flattool.Warehouse
    io.missioncenter.MissionCenter md.obsidian.Obsidian
    org.filezillaproject.Filezilla org.gaphor.Gaphor org.gnome.Boxes
    org.libreoffice.LibreOffice org.onlyoffice.desktopeditors org.qbittorrent.qBittorrent
    org.remmina.Remmina org.telegram.desktop org.videolan.VLC me.iepure.devtoolbox
    com.rtosta.zapzap
    
    #com.mikrotik.WinBox
  )
fi

FLATPAKS_JOGOS=(
  com.heroicgameslauncher.hgl com.valvesoftware.Steam
  com.vysp3r.ProtonPlus org.prismlauncher.PrismLauncher
)
