package modules

import (
	"path/filepath"

	"github.com/vinicgobbi/post-install/internal/runner"
)

const gradiaFlatpak = "be.alexandervanhee.gradia"

var atalhosGnome = Module{
	ID:    "atalhos_gnome",
	Title: "Atalhos do GNOME",
	Desc:  "Aplica GNOME/atalhos.dconf: Print e Shift+Super+S abrem o Gradia, gravação de tela em Shift+Super+R e volume de 2 em 2%",
	Run: func(c *Ctx) error {
		conf := filepath.Join(c.GNOMEDir, "atalhos.dconf")
		if !fileExists(conf) {
			c.Log.Warnf("%s não encontrado: pulando atalhos.", conf)
			return nil
		}
		if err := garantirFlatpak(c, gradiaFlatpak, "Gradia"); err != nil {
			return err
		}
		if err := c.AsUser("dconf load /org/gnome/ < " + shQuote(conf)); err != nil {
			return err
		}
		c.Log.Successf("Atalhos do GNOME aplicados.")
		return nil
	},
}

// garantirFlatpak instala um app chamado por atalho (ex.: o Gradia, que
// substitui a captura nativa no Print) se ainda não estiver: ele precisa
// existir mesmo quando o módulo de Flatpaks não foi marcado.
func garantirFlatpak(c *Ctx, app, nome string) error {
	if runner.Exists("flatpak") && c.Probe("flatpak", "info", app) {
		return nil
	}
	c.Log.Infof("%s não instalado: instalando via Flatpak para os atalhos...", nome)
	if !runner.Exists("flatpak") {
		if err := c.install("flatpak"); err != nil {
			return err
		}
	}
	return step(c.ensureFlathub, func() error { return c.flatpakInstall(app) })
}
