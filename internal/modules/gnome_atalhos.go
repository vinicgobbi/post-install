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
		if c.Sys.IsKDE() {
			c.Log.Infof("Ambiente Plasma detectado: pulando atalhos do GNOME.")
			return nil
		}
		conf := filepath.Join(c.GNOMEDir, "atalhos.dconf")
		if !fileExists(conf) {
			c.Log.Warnf("%s não encontrado: pulando atalhos.", conf)
			return nil
		}

		// O Print deixa de abrir a captura nativa, então o Gradia precisa
		// estar instalado mesmo quando o módulo de Flatpaks não foi marcado.
		if !runner.Exists("flatpak") || !c.Probe("flatpak", "info", gradiaFlatpak) {
			c.Log.Infof("Gradia não instalado: instalando via Flatpak para os atalhos de captura...")
			if !runner.Exists("flatpak") {
				if err := c.install("flatpak"); err != nil {
					return err
				}
			}
			if err := step(c.ensureFlathub, func() error { return c.flatpakInstall(gradiaFlatpak) }); err != nil {
				return err
			}
		}

		if err := c.AsUser("dconf load /org/gnome/ < " + shQuote(conf)); err != nil {
			return err
		}
		c.Log.Successf("Atalhos do GNOME aplicados.")
		return nil
	},
}
