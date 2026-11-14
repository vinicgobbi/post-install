package modules

import (
	_ "embed"
)

// O Flatpak não expõe o /usr do host, então os apps Flatpak só enxergam os
// temas publicados no Flathub (org.gtk.Gtk3theme.*), onde faltam as
// variantes de cor do Yaru. O script embutido mantém uma cópia dos temas em
// yaruFlatpakDir (sincronizada por um hook do APT a cada atualização), que é
// exposta a todos os apps; os links em ~/.local/share/themes fazem o GTK de
// dentro do sandbox encontrá-la pelo xdg-data/themes.
const (
	yaruFlatpakDir    = "/var/lib/yaru-flatpak/themes"
	yaruFlatpakScript = "/usr/local/sbin/yaru-flatpak-sync"
	yaruFlatpakHook   = "/etc/apt/apt.conf.d/99yaru-flatpak"
)

//go:embed yaru-flatpak-sync.sh
var yaruFlatpakSync string

const yaruFlatpakHookConf = `// Sincroniza os temas Yaru com a cópia que os apps Flatpak enxergam — post-install
DPkg::Post-Invoke { "if [ -x ` + yaruFlatpakScript + ` ]; then ` + yaruFlatpakScript + ` || true; fi"; };
`

// scriptLinksYaru liga cada tema da cópia em ~/.local/share/themes, sem
// mexer em temas de mesmo nome que o usuário tenha posto ali de verdade, e
// aplica os ícones Yaru-dark.
const scriptLinksYaru = `mkdir -p "$HOME/.local/share/themes"
for tema in ` + yaruFlatpakDir + `/Yaru*; do
    [ -d "$tema" ] || continue
    link="$HOME/.local/share/themes/$(basename "$tema")"
    if [ -L "$link" ] || [ ! -e "$link" ]; then
        ln -sfn "$tema" "$link"
    fi
done
gsettings set org.gnome.desktop.interface icon-theme "Yaru-dark" 2>/dev/null || true
`

var temasYaru = Module{
	ID:    "temas_yaru",
	Grupo: GrupoGNOME,
	Title: "Temas Yaru (com Flatpak)",
	Desc:  "Temas e ícones Yaru (ícones em Yaru-dark) e um hook do APT que sincroniza todas as variantes do tema com os apps Flatpak",
	Run:   instalarYaru,
	// No Fedora, só os ícones são do Yaru; o tema segue o libadwaita, com o
	// adw-gtk3 (o visual do libadwaita para os apps GTK 3) no sistema e nos
	// Flatpaks, pelo Flathub. Os ícones do sistema os Flatpaks já enxergam.
	// O GNOME puro também só mostra o botão de fechar nas janelas: os de
	// minimizar e maximizar voltam, como no Ubuntu.
	Fedora: &Variante{
		Title: "Ícones Yaru, tema adw-gtk3 e botões das janelas",
		Desc:  "Ícones Yaru (Yaru-dark) e o tema adw-gtk3-dark (o visual do libadwaita nos apps GTK 3), também nos apps Flatpak, e os botões de minimizar e maximizar nas janelas",
		Run:   aplicarYaruAdwGtk3,
	},
	// O Mint e o Zorin têm a identidade visual deles (Mint-Y e os temas do
	// Zorin): o post-install não mexe no tema nem nos ícones.
	Mint:  &Variante{Indisponivel: true},
	Zorin: &Variante{Indisponivel: true},
}

// instalarYaru instala os temas Yaru e os sincroniza com os Flatpaks.
func instalarYaru(c *Ctx) error {
	c.Log.Infof("Instalando os temas e ícones Yaru...")
	err := step(
		func() error { return c.install("yaru-theme-gtk", "yaru-theme-icon", "flatpak") },
		func() error { return c.WriteFile(yaruFlatpakScript, yaruFlatpakSync, 0o755) },
		func() error { return c.WriteFile(yaruFlatpakHook, yaruFlatpakHookConf, 0o644) },
		func() error { return c.Run(yaruFlatpakScript, "--force") },
		func() error {
			return c.Run("flatpak", "override", "--system",
				"--filesystem=xdg-data/themes:ro", "--filesystem="+yaruFlatpakDir+":ro")
		},
		func() error { return c.AsUser(scriptLinksYaru) },
	)
	if err != nil {
		return err
	}
	c.Log.Successf("Temas e ícones Yaru instalados; as variantes do tema passam a valer nos apps Flatpak (GTK 3) a partir da próxima abertura de cada app.")
	return nil
}

// scriptIconesAdwGtk3 aplica os ícones Yaru-dark e o adw-gtk3-dark (escuro,
// combinando com os ícones) como o usuário, e põe os botões de minimizar e
// maximizar ao lado do de fechar (o padrão do Fedora é "appmenu:close").
const scriptIconesAdwGtk3 = `gsettings set org.gnome.desktop.interface icon-theme "Yaru-dark"
gsettings set org.gnome.desktop.interface gtk-theme "adw-gtk3-dark"
gsettings set org.gnome.desktop.wm.preferences button-layout "appmenu:minimize,maximize,close"
`

func aplicarYaruAdwGtk3(c *Ctx) error {
	c.Log.Infof("Instalando os ícones Yaru e o tema adw-gtk3...")
	err := step(
		func() error { return c.install("yaru-icon-theme", "adw-gtk3-theme") },
		func() error { return c.garantir("flatpak", "flatpak") },
		c.garantirFlathub,
		func() error { return c.flatpakInstall("org.gtk.Gtk3theme.adw-gtk3", "org.gtk.Gtk3theme.adw-gtk3-dark") },
		func() error { return c.AsUser(scriptIconesAdwGtk3) },
	)
	if err != nil {
		return err
	}
	c.Log.Successf("Ícones Yaru-dark, tema adw-gtk3-dark (nos apps Flatpak, a partir da próxima abertura de cada app) e botões de minimizar e maximizar aplicados.")
	return nil
}

// scriptPastaProjetos cria a pasta de projetos (no idioma do sistema),
// registra como diretório XDG e fixa na barra lateral do Nautilus.
const scriptPastaProjetos = `
if [[ "$LANG" == pt_* ]]; then DIR_NAME="Projetos"; else DIR_NAME="Projects"; fi
PROJECTS_DIR="$HOME/$DIR_NAME"
mkdir -p "$PROJECTS_DIR"
xdg-user-dirs-update --set PROJECTS "$PROJECTS_DIR"

BOOKMARKS_FILE="$HOME/.config/gtk-3.0/bookmarks"
mkdir -p "$(dirname "$BOOKMARKS_FILE")"
touch "$BOOKMARKS_FILE"
grep -q "file://$PROJECTS_DIR" "$BOOKMARKS_FILE" || echo "file://$PROJECTS_DIR" >> "$BOOKMARKS_FILE"
`

var nautilus = Module{
	ID:    "nautilus",
	Grupo: GrupoGNOME,
	Title: "Nautilus: VSCode, copiar caminho e Projetos",
	Desc:  "Adiciona 'Abrir com o VSCode' e 'Copiar caminho' ao menu de contexto do Nautilus e cria a pasta Projetos (diretório XDG e favorito na barra lateral)",
	Run: func(c *Ctx) error {
		return instalarNautilus(c, "python3-nautilus", "python3-gi", "wl-clipboard", "xclip", "git")
	},
	Fedora: &Variante{
		Run: func(c *Ctx) error {
			return instalarNautilus(c, "nautilus-python", "python3-gobject", "wl-clipboard", "xclip", "git")
		},
	},
	// O gerenciador de arquivos do Mint é o Nemo, que não carrega as
	// extensões em Python do Nautilus: as mesmas opções viram ações do Nemo
	// (arquivos .nemo_action do usuário).
	Mint: &Variante{
		Title: "Nemo: VSCode, copiar caminho e Projetos",
		Desc:  "Adiciona 'Abrir com o VSCode' e 'Copiar caminho' ao menu de contexto do Nemo (ações do Nemo) e cria a pasta Projetos (diretório XDG e favorito na barra lateral)",
		Run:   instalarAcoesNemo,
	},
}

// scriptAcoesNemo grava as ações do usuário em ~/.local/share/nemo/actions:
// abrir a seleção (ou, clicando no fundo, a pasta atual) no VSCode e copiar
// os caminhos da seleção, um por linha, com o wl-copy no Wayland ou o xclip
// no X11. Quote=double: o Nemo põe cada caminho entre aspas no Exec.
const scriptAcoesNemo = `set -e
dir="$HOME/.local/share/nemo/actions"
mkdir -p "$dir"
cat > "$dir/post-install-vscode.nemo_action" <<'ACAO'
[Nemo Action]
Name=Abrir com o VSCode
Comment=Abre os itens selecionados no Visual Studio Code
Exec=code %F
Icon-Name=vscode
Selection=notnone
Extensions=any;
Quote=double
Dependencies=code;
ACAO
cat > "$dir/post-install-vscode-pasta.nemo_action" <<'ACAO'
[Nemo Action]
Name=Abrir com o VSCode
Comment=Abre a pasta atual no Visual Studio Code
Exec=code %P
Icon-Name=vscode
Selection=none
Extensions=any;
Quote=double
Dependencies=code;
ACAO
cat > "$dir/post-install-copiar-caminho.sh" <<'ACAO'
#!/bin/sh
# Copia os caminhos recebidos, um por linha (ação do Nemo do post-install).
if [ -n "${WAYLAND_DISPLAY:-}" ] && command -v wl-copy >/dev/null 2>&1; then
    printf '%s\n' "$@" | head -c -1 | wl-copy
else
    printf '%s\n' "$@" | head -c -1 | xclip -selection clipboard
fi
ACAO
chmod 0755 "$dir/post-install-copiar-caminho.sh"
cat > "$dir/post-install-copiar-caminho.nemo_action" <<ACAO
[Nemo Action]
Name=Copiar caminho
Comment=Copia o caminho dos itens selecionados
Exec=$dir/post-install-copiar-caminho.sh %F
Icon-Name=edit-copy
Selection=notnone
Extensions=any;
Quote=double
ACAO
`

func instalarAcoesNemo(c *Ctx) error {
	c.Log.Infof("Adicionando as ações do VSCode e de copiar caminho ao Nemo...")
	err := step(
		func() error { return c.install("wl-clipboard", "xclip") },
		func() error { return c.AsUser(scriptAcoesNemo) },
		func() error { return c.AsUser(scriptPastaProjetos) },
	)
	if err != nil {
		return err
	}
	c.Log.Successf("Ações do Nemo instaladas e pasta de projetos criada.")
	return nil
}

func instalarNautilus(c *Ctx, deps ...string) error {
	c.Log.Infof("Instalando extensões do VSCode e de copiar caminho para o Nautilus...")
	err := step(
		func() error { return c.install(deps...) },
		func() error {
			return c.AsUser("set -e\ndir=$(mktemp -d)\ntrap 'rm -rf \"$dir\"' EXIT\n" +
				clonarFixado(vscodeNautilusRepo, vscodeNautilusCommit, `"$dir/vscode_nautilus"`) +
				`bash "$dir/vscode_nautilus/install.sh"` + "\n" +
				clonarFixado(copypathRepo, copypathCommit, `"$dir/nautilus-copypath"`) +
				`bash "$dir/nautilus-copypath/install.sh"` + "\n")
		},
		func() error { return c.AsUser(scriptPastaProjetos) },
	)
	if err != nil {
		return err
	}
	c.Log.Successf("Extensões do Nautilus instaladas e pasta de projetos criada.")
	return nil
}
