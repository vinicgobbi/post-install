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
	Run: func(c *Ctx) error {
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
	},
	// No Fedora, só os ícones são do Yaru; o tema segue o libadwaita, com o
	// adw-gtk3 (o visual do libadwaita para os apps GTK 3) no sistema e nos
	// Flatpaks, pelo Flathub. Os ícones do sistema os Flatpaks já enxergam.
	Fedora: &Variante{
		Title: "Ícones Yaru e tema adw-gtk3",
		Desc:  "Ícones Yaru (Yaru-dark) e o tema adw-gtk3-dark (o visual do libadwaita nos apps GTK 3), também nos apps Flatpak",
		Run:   aplicarYaruAdwGtk3,
	},
}

// scriptIconesAdwGtk3 aplica os ícones Yaru-dark e o adw-gtk3-dark (escuro,
// combinando com os ícones) como o usuário.
const scriptIconesAdwGtk3 = `gsettings set org.gnome.desktop.interface icon-theme "Yaru-dark"
gsettings set org.gnome.desktop.interface gtk-theme "adw-gtk3-dark"
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
	c.Log.Successf("Ícones Yaru-dark e tema adw-gtk3-dark aplicados (nos apps Flatpak, a partir da próxima abertura de cada app).")
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
