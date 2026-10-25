package modules

import (
	_ "embed"
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/vinicgobbi/post-install/internal/config"
	"github.com/vinicgobbi/post-install/internal/runner"
)

var solaar = Module{
	ID:    "solaar",
	Title: "Instalar e configurar Solaar",
	Desc:  "Instala o Solaar (PPA solaar-unifying/stable, com fallback para Flatpak) e as regras UDEV do receptor Logitech Unifying",
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
	Title: "Instalar Flatpaks",
	Desc:  "Instala a lista de aplicativos Flatpak (Postman, Spotify, Obsidian, etc.)",
	Run: func(c *Ctx) error {
		c.Log.Infof("Configurando Flatpak e instalando apps...")
		if err := step(c.ensureFlathub, func() error { return c.flatpakInstall(config.Flatpaks...) }); err != nil {
			return err
		}
		c.Log.Successf("Aplicativos Flatpak instalados.")
		return nil
	},
}

var flatpaksJogos = Module{
	ID:    "flatpaks_jogos",
	Title: "Instalar Flatpaks de jogos",
	Desc:  "Steam, Heroic Games Launcher, ProtonPlus e PrismLauncher",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando Flatpaks de jogos...")
		if err := step(c.ensureFlathub, func() error { return c.flatpakInstall(config.FlatpaksJogos...) }); err != nil {
			return err
		}
		c.Log.Successf("Flatpaks de jogos instalados.")
		return nil
	},
}

// Chave que assina os repositórios de pacotes do Tailscale.
const fprTailscale = "2596A99EAAB33821893C0A79458CA832957F5868"

// instalarTailscalePacote configura o repositório oficial do Tailscale
// (pkgs.tailscale.com, assinado) e instala o pacote — o mesmo que o
// install.sh deles faz, mas sem executar um script baixado como root.
func instalarTailscalePacote(c *Ctx) error {
	const (
		base    = "https://pkgs.tailscale.com/stable/ubuntu"
		keyring = "/usr/share/keyrings/tailscale-archive-keyring.gpg"
	)
	codename := c.Sys.Codename
	return step(
		func() error {
			err := instalarChaveAPT(c, base+"/"+codename+".noarmor.gpg", keyring, fprTailscale)
			if err != nil {
				return errorf("repositório do Tailscale indisponível para o Ubuntu %s: %w", codename, err)
			}
			return nil
		},
		func() error {
			return c.WriteFile("/etc/apt/sources.list.d/tailscale.list",
				"deb [signed-by="+keyring+"] "+base+" "+codename+" main\n", 0o644)
		},
		func() error { return c.Run("apt", "update") },
		func() error { return c.install("tailscale") },
	)
}

var tailscale = Module{
	ID:    "tailscale",
	Title: "Instalar Tailscale + Trayscale",
	Desc:  "Tailscale (VPN mesh, pelo repositório oficial) e o cliente gráfico Trayscale via Flatpak",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando o Tailscale e o Trayscale...")
		err := step(
			func() error { return instalarTailscalePacote(c) },
			func() error { return c.Run("systemctl", "enable", "--now", "tailscaled") },
			c.ensureFlathub,
			func() error { return c.flatpakInstall("dev.deedles.Trayscale") },
			func() error { return c.Run("tailscale", "set", "--operator="+c.User) },
		)
		if err != nil {
			return err
		}
		c.Log.Successf("Tailscale instalado e '%s' definido como operator.", c.User)
		return nil
	},
}

var phpExtensoes = Module{
	ID:    "php_extensoes",
	Title: "Extensões PHP (SQL Server)",
	Desc:  "Compila e habilita sqlsrv/pdo_sqlsrv via PECL",
	Deps:  []string{"pacotes_base"},
	Run: func(c *Ctx) error {
		c.Log.Infof("Configurando extensões PHP (sqlsrv)...")
		err := step(
			func() error { return c.Run("pecl", "install", "sqlsrv", "pdo_sqlsrv") },
			func() error {
				out, err := c.Output("php", "-r", `echo PHP_MAJOR_VERSION.".".PHP_MINOR_VERSION;`)
				if err != nil {
					return err
				}
				dir := "/etc/php/" + strings.TrimSpace(out) + "/mods-available/"
				return step(
					func() error { return c.WriteFile(dir+"sqlsrv.ini", "; priority=20\nextension=sqlsrv.so\n", 0o644) },
					func() error {
						return c.WriteFile(dir+"pdo_sqlsrv.ini", "; priority=20\nextension=pdo_sqlsrv.so\n", 0o644)
					},
					func() error { return c.Run("phpenmod", "sqlsrv", "pdo_sqlsrv") },
				)
			},
		)
		if err != nil {
			return err
		}
		c.Log.Successf("Extensões PHP configuradas.")
		return nil
	},
}

// releaseAsset é um arquivo de uma release do GitHub. SHA256 vem do campo
// "digest" que o GitHub calcula no upload (vazio em releases antigas).
type releaseAsset struct {
	Name   string
	URL    string
	SHA256 string
}

// latestReleaseAsset devolve o primeiro asset da última release de um
// repositório do GitHub que satisfaça match.
func latestReleaseAsset(c *Ctx, repo string, match func(name string) bool) (releaseAsset, error) {
	body, err := c.Get("https://api.github.com/repos/"+repo+"/releases/latest", 30*time.Second)
	if err != nil {
		return releaseAsset{}, err
	}
	var rel struct {
		Assets []struct {
			Name   string `json:"name"`
			URL    string `json:"browser_download_url"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return releaseAsset{}, err
	}
	for _, a := range rel.Assets {
		if match(a.Name) {
			sha, _ := strings.CutPrefix(a.Digest, "sha256:")
			return releaseAsset{Name: a.Name, URL: a.URL, SHA256: sha}, nil
		}
	}
	return releaseAsset{}, errorf("nenhum asset compatível na última release de %s", repo)
}

// downloadAsset baixa um asset de release conferindo o SHA-256 publicado
// pelo GitHub, quando houver.
func downloadAsset(c *Ctx, a releaseAsset, dest string) error {
	if a.SHA256 == "" {
		c.Log.Warnf("%s não tem SHA-256 publicado na release: baixando sem conferência de integridade.", a.Name)
		return c.Download(a.URL, dest)
	}
	return c.DownloadVerified(a.URL, dest, a.SHA256)
}

func gcmDeb(name string) bool {
	return strings.HasSuffix(name, ".deb") && strings.Contains(name, "linux-x64")
}

var chromeGCM = Module{
	ID:    "chrome_gcm",
	Title: "Chrome + Git Credential Manager",
	Desc:  "Instala o Google Chrome e o Git Credential Manager",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando Git Credential Manager e Google Chrome...")
		const gcmRepo = "git-ecosystem/git-credential-manager"
		asset, err := latestReleaseAsset(c, gcmRepo, gcmDeb)
		if err != nil {
			return err
		}
		gcm, chrome := c.Temp("gcm.deb"), c.Temp("chrome.deb")
		// Baixa os dois .deb em paralelo e instala ambos numa única chamada
		// do apt (evita duas resoluções de dependência). O .deb do Chrome não
		// é assinado individualmente, mas ao ser instalado ele configura o
		// repositório assinado do Google, por onde vêm as atualizações.
		var g errgroup.Group
		g.Go(func() error { return downloadAsset(c, asset, gcm) })
		g.Go(func() error {
			return c.Download("https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb", chrome)
		})
		if err := g.Wait(); err != nil {
			return err
		}
		if err := c.install(gcm, chrome); err != nil {
			return err
		}
		c.Log.Successf("Chrome e GCM instalados.")
		return nil
	},
}

var bitwarden = Module{
	ID:    "bitwarden",
	Title: "Instalar Bitwarden nativo",
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
