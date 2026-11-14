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
