#!/usr/bin/env bash

instalar_vscode_gerenciador_arquivos() {
    if [[ "$DESKTOP_ENV" == "kde" ]]; then
        info "Ambiente Plasma detectado: adicionando 'Abrir com o VSCode' ao menu de contexto do Dolphin..."

        mkdir -p /usr/share/kio/servicemenus
        cat > /usr/share/kio/servicemenus/vscode-open-with.desktop <<'EOF'
[Desktop Entry]
Type=Service
X-KDE-ServiceTypes=KonqPopupMenu/Plugin
MimeType=all/allfiles;inode/directory;
Actions=openWithCode;

[Desktop Action openWithCode]
Name=Abrir com o VSCode
Icon=code
Exec=code %U
EOF

        command -v kbuildsycoca6 >/dev/null 2>&1 && kbuildsycoca6 --noincremental >/dev/null 2>&1
        command -v kbuildsycoca5 >/dev/null 2>&1 && kbuildsycoca5 --noincremental >/dev/null 2>&1

        sucesso "Item 'Abrir com o VSCode' adicionado ao Dolphin."
        return
    fi

    info "Instalando extensões do VSCode e de copiar caminho para o Nautilus..."

    if [[ "$PKG_MGR" == "dnf" ]]; then
        dnf install -y nautilus-python python3-gobject wl-clipboard xclip
    elif [[ "$PKG_MGR" == "pacman" ]]; then
        pacman -S --needed --noconfirm nautilus-python python-gobject wl-clipboard xclip
    else
        apt install -y python3-nautilus python3-gi wl-clipboard xclip
    fi

    executar_como_usuario "
  rm -rf /tmp/vscode_nautilus
  git clone https://github.com/vinicgobbi/vscode_nautilus.git /tmp/vscode_nautilus
  bash /tmp/vscode_nautilus/install.sh

  rm -rf /tmp/nautilus-copypath
  git clone https://github.com/vinicgobbi/nautilus-copypath.git /tmp/nautilus-copypath
  bash /tmp/nautilus-copypath/install.sh
"
    sucesso "Extensões do VSCode e de copiar caminho instaladas no Nautilus."
}

registrar_modulo "vscode_nautilus" "Extensão VSCode no gerenciador de arquivos" \
    "Adiciona 'Abrir com o VSCode' ao menu de contexto (Nautilus no GNOME, Dolphin no Plasma) e, no Nautilus, 'Copiar caminho'" \
    "instalar_vscode_gerenciador_arquivos"
