package modules

import (
	_ "embed"
	"path/filepath"

	"github.com/vinicgobbi/post-install/internal/config"
	"github.com/vinicgobbi/post-install/internal/runner"
)

var solaar = Module{
	ID:    "solaar",
	Grupo: GrupoAplicativos,
	Title: "Solaar (Logitech)",
	Desc:  "Instala o Solaar (PPA solaar-unifying/stable, com fallback para Flatpak), as regras UDEV do receptor Logitech Unifying e o início junto da sessão, só na bandeja",
	Run: func(c *Ctx) error {
		if err := instalarSolaar(c); err != nil {
			return err
		}

		if !c.Probe("getent", "group", "plugdev") {
			if err := c.Run("groupadd", "plugdev"); err != nil {
				return err
			}
		}
		if err := c.Run("usermod", "-aG", "plugdev", c.User); err != nil {
			return err
		}
		if err := c.AsUser(scriptAutostartSolaar); err != nil {
			c.Log.Warnf("Não foi possível pôr o Solaar para iniciar com a sessão: %v", err)
		}

		// O pacote da PPA já traz a regra em /usr/lib/udev/rules.d; só
		// o Flatpak (que não instala nada fora do sandbox) precisa dela aqui.
		if regraSolaarInstalada() {
			c.Log.Infof("Regra UDEV do Solaar já instalada pelo pacote.")
			return nil
		}
		c.Log.Infof("Configurando regras UDEV para o Solaar (regra do Solaar %s)...", solaarTag)
		err := step(
			func() error { return c.WriteFile(regraSolaarDestino, regraSolaar, 0o644) },
			func() error { return c.Run("udevadm", "control", "--reload-rules") },
			func() error { return c.Run("udevadm", "trigger", "--subsystem-match=usb") },
			func() error { return c.Run("udevadm", "trigger", "--subsystem-match=hidraw") },
		)
		if err != nil {
			return err
		}
		c.Log.Successf("Regras UDEV do Solaar configuradas.")
		return nil
	},
}

// scriptAutostartSolaar põe o Solaar (pacote, ou Flatpak como alternativa)
// para iniciar com a sessão só no ícone da bandeja ("-w hide"), sem abrir a
// janela.
const scriptAutostartSolaar = `mkdir -p "$HOME/.config/autostart"
for desktop in /usr/share/applications/solaar.desktop \
    /var/lib/flatpak/exports/share/applications/io.github.pwr_solaar.solaar.desktop; do
    if [ -f "$desktop" ]; then
        destino="$HOME/.config/autostart/$(basename "$desktop")"
        cp "$desktop" "$destino"
        sed -i -E '/^Exec=/{/ -w hide$/! s/$/ -w hide/}' "$destino"
        break
    fi
done
`

const regraSolaarNome = "42-logitech-unify-permissions.rules"

var regraSolaarDestino = "/etc/udev/rules.d/" + regraSolaarNome

// solaarTag é a versão do Solaar de onde veio a regra embutida abaixo
// (rules.d-uinput/42-logitech-unify-permissions.rules). Ela vem no binário
// em vez de baixada: regras UDEV rodam como root (podem ter RUN+=), então não
// convém buscá-las de uma branch que muda. O workflow pins.yml atualiza as
// duas coisas juntas quando sai versão nova.
const solaarTag = "1.1.20"

//go:embed 42-logitech-unify-permissions.rules
var regraSolaar string

// regraSolaarInstalada diz se algum pacote já instalou a regra do Solaar.
func regraSolaarInstalada() bool {
	for _, dir := range []string{"/usr/lib/udev/rules.d", "/lib/udev/rules.d"} {
		if fileExists(filepath.Join(dir, regraSolaarNome)) {
			return true
		}
	}
	return false
}

func instalarSolaar(c *Ctx) error {
	c.Log.Infof("Instalando Solaar pela PPA solaar-unifying/stable (sempre atualizada)...")
	err := step(
		func() error {
			if runner.Exists("add-apt-repository") {
				return nil
			}
			return c.install("software-properties-common")
		},
		func() error { return c.Run("add-apt-repository", "-y", "ppa:solaar-unifying/stable") },
		func() error { return c.Run("apt", "update") },
		func() error { return c.install("solaar") },
	)
	if err == nil {
		c.Log.Successf("Solaar instalado via PPA solaar-unifying/stable.")
		return nil
	}

	c.Log.Warnf("Falha ao instalar o Solaar pela PPA (%v); instalando via Flatpak.", err)
	if err := step(c.ensureFlathub, func() error { return c.flatpakInstall("io.github.pwr_solaar.solaar") }); err != nil {
		return err
	}
	c.Log.Successf("Solaar instalado via Flatpak.")
	return nil
}

var removerLibreoffice = Module{
	ID:    "remover_libreoffice",
	Grupo: GrupoAplicativos,
	Title: "Remover LibreOffice nativo",
	Desc:  "Remove o LibreOffice do sistema (será substituído pela versão Flatpak)",
	Run: func(c *Ctx) error {
		c.Log.Infof("Removendo LibreOffice pré-instalado...")
		if err := c.Run("apt", "remove", "--purge", "-y", "libreoffice*"); err != nil {
			return err
		}
		c.Log.Successf("LibreOffice nativo removido.")
		return nil
	},
}

var flatpaks = Module{
	ID:    "flatpaks",
	Grupo: GrupoAplicativos,
	Title: "Flatpaks",
	Desc:  "Flathub, GNOME Software com suporte a Flatpak e a lista de aplicativos (Postman, Spotify, Obsidian, etc.)",
	Run: func(c *Ctx) error {
		c.Log.Infof("Configurando Flatpak e instalando apps...")
		if err := step(
			func() error {
				return c.install("flatpak", "gnome-software", "gnome-software-plugin-flatpak", "gnome-software-plugin-snap")
			},
			c.ensureFlathub, func() error { return c.flatpakInstall(config.Flatpaks...) }); err != nil {
			return err
		}
		c.Log.Successf("Aplicativos Flatpak instalados.")
		return nil
	},
}

var flatpaksJogos = Module{
	ID:    "flatpaks_jogos",
	Grupo: GrupoAplicativos,
	Title: "Flatpaks de jogos",
	Desc:  "Steam, Heroic Games Launcher, ProtonPlus e PrismLauncher",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando Flatpaks de jogos...")
		if err := step(
			func() error { return c.garantir("flatpak", "flatpak") },
			c.ensureFlathub,
			func() error { return c.flatpakInstall(config.FlatpaksJogos...) },
		); err != nil {
			return err
		}
		c.Log.Successf("Flatpaks de jogos instalados.")
		return nil
	},
}

var bitwarden = Module{
	ID:    "bitwarden",
	Grupo: GrupoAplicativos,
	Title: "Bitwarden",
	Desc:  "Instala o Bitwarden desktop (necessário para integração nativa com o navegador)",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando Bitwarden nativo (integração com navegador)...")
		pkg := c.Temp("bitwarden.deb")
		err := step(
			func() error {
				return c.Download("https://vault.bitwarden.com/download/?app=desktop&platform=linux&variant=deb", pkg)
			},
			func() error { return c.install(pkg) },
		)
		if err != nil {
			return err
		}
		c.Log.Successf("Bitwarden instalado nativamente.")
		return nil
	},
}

var chrome = Module{
	ID:    "chrome",
	Grupo: GrupoAplicativos,
	Title: "Google Chrome",
	Desc:  "Instala o Google Chrome pelo .deb oficial, que configura o repositório assinado do Google para as atualizações",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando o Google Chrome...")
		// O .deb do Chrome não é assinado individualmente, mas ao ser
		// instalado ele configura o repositório assinado do Google, por onde
		// vêm as atualizações.
		deb := c.Temp("chrome.deb")
		err := step(
			func() error {
				return c.Download("https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb", deb)
			},
			func() error { return c.install(deb) },
		)
		if err != nil {
			return err
		}
		c.Log.Successf("Google Chrome instalado.")
		return nil
	},
}
