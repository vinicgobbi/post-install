package modules

import (
	"path/filepath"
)

const gradiaFlatpak = "be.alexandervanhee.gradia"

var atalhosGnome = Module{
	ID:    "atalhos_gnome",
	Grupo: GrupoGNOME,
	Title: "Atalhos do GNOME",
	Desc:  "Aplica GNOME/atalhos.dconf: Print e Shift+Super+S abrem o Gradia, gravação de tela em Shift+Super+R e volume de 2 em 2%",
	Run: func(c *Ctx) error {
		return aplicarAtalhos(c, "atalhos.dconf", "/org/gnome/", "GNOME")
	},
	Arch: &Variante{},
	// No KDE, o Print continua com o Spectacle; ele ganha os atalhos que o
	// GNOME tem aqui: Meta+Shift+S (região) e Meta+Shift+R (gravação). O
	// volume muda de 2 em 2%, como o volume-step do atalhos.dconf, Meta+.
	// abre o seletor de emojis e as bordas da tela (cantos ativos) ficam
	// desligadas.
	KDE: &Variante{
		Title: "Atalhos do KDE",
		Desc:  "Meta+Shift+S captura uma região e Meta+Shift+R grava uma região com o Spectacle (o Print continua com o Spectacle), Meta+. abre o seletor de emojis, volume de 2 em 2% e bordas da tela desativadas",
		Run:   aplicarAtalhosKDE,
	},
	Fedora: &Variante{},
	// O Cinnamon guarda os atalhos em outro lugar e em outro formato (o
	// binding é uma lista), daí o arquivo próprio.
	Mint: &Variante{
		Title: "Atalhos do Cinnamon",
		Desc:  "Aplica GNOME/atalhos-cinnamon.dconf: Print e Shift+Super+S abrem o Gradia",
		Run: func(c *Ctx) error {
			return aplicarAtalhos(c, "atalhos-cinnamon.dconf", "/org/cinnamon/", "Cinnamon")
		},
	},
}

// aplicarAtalhos carrega o arquivo de GNOMEDir no caminho do dconf dado,
// depois de garantir o Gradia, que os atalhos chamam.
func aplicarAtalhos(c *Ctx, arquivo, caminho, desktop string) error {
	conf := filepath.Join(c.GNOMEDir, arquivo)
	if !c.FileExists(conf) {
		c.Log.Warnf("%s não encontrado: pulando atalhos.", conf)
		return nil
	}
	if err := garantirFlatpak(c, gradiaFlatpak, "Gradia"); err != nil {
		return err
	}
	if err := c.AsUser("dconf load " + caminho + " < " + shQuote(conf)); err != nil {
		return err
	}
	c.Log.Successf("Atalhos do %s aplicados.", desktop)
	return nil
}

// garantirFlatpak instala um app chamado por atalho (ex.: o Gradia, que
// substitui a captura nativa no Print) se ainda não estiver: ele precisa
// existir mesmo quando o módulo de Flatpaks não foi marcado.
func garantirFlatpak(c *Ctx, app, nome string) error {
	if c.Exists("flatpak") && c.Probe("flatpak", "info", app) {
		return nil
	}
	c.Log.Infof("%s não instalado: instalando via Flatpak para os atalhos...", nome)
	if !c.Exists("flatpak") {
		if err := c.install("flatpak"); err != nil {
			return err
		}
	}
	return step(c.garantirFlathub, func() error { return c.flatpakInstall(app) })
}

// scriptAtalhosKDE grava os atalhos do Spectacle no kglobalshortcutsrc
// (grupo [services][org.kde.spectacle.desktop] do Plasma 6; vários atalhos
// para a mesma ação são separados por tabulação). A captura de região mantém
// o padrão (Meta+Shift+Print) e ganha o Meta+Shift+S. O seletor de emojis
// (plasma-emojier) fica com o Meta+. e mantém o atalho alternativo padrão.
//
// Bordas da tela: no Plasma 6, só o canto superior esquerdo vem ativo (abre
// a Visão geral); BorderActivate=9 é "nenhuma borda" no KWin. As ações dos
// cantos ([ElectricBorders]) também ficam em None, caso alguma tenha sido
// ligada. O
// reconfigure aplica na hora, se o KWin estiver rodando.
const scriptAtalhosKDE = `set -e
kwriteconfig6 --file kglobalshortcutsrc --group services --group org.kde.spectacle.desktop --key RectangularRegionScreenShot "$(printf 'Meta+Shift+Print\tMeta+Shift+S')"
kwriteconfig6 --file kglobalshortcutsrc --group services --group org.kde.spectacle.desktop --key RecordRegion "Meta+Shift+R"
kwriteconfig6 --file kglobalshortcutsrc --group services --group org.kde.plasma.emojier.desktop --key _launch "$(printf 'Meta+.\tMeta+Ctrl+Alt+Shift+Space')"
kwriteconfig6 --file plasmaparc --group General --key VolumeStep 2
kwriteconfig6 --file kwinrc --group Effect-overview --key BorderActivate 9
kwriteconfig6 --file kwinrc --group Effect-overview --key TouchBorderActivate 9
for canto in Top TopRight Right BottomRight Bottom BottomLeft Left TopLeft; do
	kwriteconfig6 --file kwinrc --group ElectricBorders --key "$canto" None
done
dbus-send --session --dest=org.kde.KWin /KWin org.kde.KWin.reconfigure 2>/dev/null || true
`

func aplicarAtalhosKDE(c *Ctx) error {
	c.Log.Infof("Configurando os atalhos do Spectacle...")
	// No Ubuntu (Kubuntu), o pacote do Spectacle é o kde-spectacle.
	pkg := "spectacle"
	if !c.fedora() && !c.arch() {
		pkg = "kde-spectacle"
	}
	if err := step(
		func() error { return c.install(pkg) },
		func() error { return c.AsUser(scriptAtalhosKDE) },
	); err != nil {
		return err
	}
	c.Log.Successf("Atalhos do KDE aplicados (Meta+Shift+S, Meta+Shift+R, Meta+. para emojis, volume de 2 em 2%% e bordas da tela desativadas); valem a partir do próximo login.")
	return nil
}
