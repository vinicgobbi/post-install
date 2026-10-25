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
	Desc:  "Instala o Solaar (PPA no Ubuntu, nativo nas demais distros, com fallback para Flatpak) e as regras UDEV do receptor Logitech Unifying",
	Run: func(c *Ctx) error {
		viaPPA := c.Sys.PkgMgr == "apt" && c.Sys.DockerDistro == "ubuntu"
		if err := instalarSolaar(c, viaPPA); err != nil {
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

		// Os pacotes nativos já trazem a regra em /usr/lib/udev/rules.d; só
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

func instalarSolaar(c *Ctx, viaPPA bool) error {
	c.Log.Infof("Instalando Solaar...")

	// No Ubuntu (e derivados), a PPA solaar-unifying/stable traz o Solaar
	// sempre atualizado.
	if viaPPA {
		c.Log.Infof("Sistema baseado em Ubuntu: usando a PPA solaar-unifying/stable.")
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
		if err != nil {
			return err
		}
		c.Log.Successf("Solaar instalado via PPA solaar-unifying/stable.")
		return nil
	}

	var disponivel bool
	switch c.Sys.PkgMgr {
	case "dnf":
		disponivel = c.Probe("dnf", "info", "solaar")
	case "pacman":
		disponivel = c.Probe("pacman", "-Si", "solaar")
	default:
		disponivel = c.Probe("apt-cache", "show", "solaar")
	}

	if disponivel {
		if err := c.install("solaar"); err != nil {
			return err
		}
		c.Log.Successf("Solaar instalado via pacote nativo (%s).", c.Sys.PkgMgr)
		return nil
	}

	c.Log.Warnf("Pacote nativo do Solaar não encontrado; instalando via Flatpak.")
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
		var err error
		switch c.Sys.PkgMgr {
		case "dnf":
			err = c.Run("dnf", "remove", "-y", "libreoffice*")
		case "pacman":
			out, _ := c.Output("pacman", "-Qq")
			var pacotes []string
			for _, p := range strings.Fields(out) {
				if strings.HasPrefix(p, "libreoffice") {
					pacotes = append(pacotes, p)
				}
			}
			if len(pacotes) == 0 {
				c.Log.Infof("Nenhum pacote libreoffice* instalado nativamente.")
			} else {
				err = c.Run("pacman", append([]string{"-Rns", "--noconfirm"}, pacotes...)...)
			}
		default:
			err = c.Run("apt", "remove", "--purge", "-y", "libreoffice*")
		}
		if err != nil {
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
	const base = "https://pkgs.tailscale.com/stable/"
	switch c.Sys.PkgMgr {
	case "pacman":
		return c.install("tailscale")
	case "dnf":
		repo := base + "fedora/"
		if c.Sys.Family == "rhel" {
			repo = base + "rhel/" + c.Sys.RHELVersion + "/"
		}
		return step(
			func() error { return importarChaveRPM(c, repo+"repo.gpg", "tailscale.gpg", fprTailscale) },
			func() error { return c.Download(repo+"tailscale.repo", "/etc/yum.repos.d/tailscale.repo") },
			func() error { return c.install("tailscale") },
		)
	default:
		const keyring = "/usr/share/keyrings/tailscale-archive-keyring.gpg"
		distro, codename := c.Sys.DockerDistro, c.Sys.BaseCodename
		return step(
			func() error {
				err := instalarChaveAPT(c, base+distro+"/"+codename+".noarmor.gpg", keyring, fprTailscale)
				if err != nil {
					return errorf("repositório do Tailscale indisponível para %s %s: %w", distro, codename, err)
				}
				return nil
			},
			func() error {
				return c.WriteFile("/etc/apt/sources.list.d/tailscale.list",
					"deb [signed-by="+keyring+"] "+base+distro+" "+codename+" main\n", 0o644)
			},
			func() error { return c.Run("apt", "update") },
			func() error { return c.install("tailscale") },
		)
	}
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
		var err error
		switch c.Sys.PkgMgr {
		case "pacman":
			// No Arch o php oficial não traz pecl/pear; os pacotes da AUR já
			// vêm pré-compilados e registram o próprio .ini em /etc/php/conf.d.
			err = c.installAUR("php-sqlsrv", "php-pdo_sqlsrv")
		case "dnf":
			err = step(
				func() error { return c.Run("pecl", "install", "sqlsrv", "pdo_sqlsrv") },
				func() error { return c.WriteFile("/etc/php.d/20-sqlsrv.ini", "extension=sqlsrv.so\n", 0o644) },
				func() error { return c.WriteFile("/etc/php.d/20-pdo_sqlsrv.ini", "extension=pdo_sqlsrv.so\n", 0o644) },
			)
		default:
			err = step(
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
		}
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

func gcmTarball(name string) bool {
	return strings.HasSuffix(name, ".tar.gz") && strings.Contains(name, "linux-x64") && !strings.Contains(name, "symbols")
}

func gcmDeb(name string) bool {
	return strings.HasSuffix(name, ".deb") && strings.Contains(name, "linux-x64")
}

func instalarGCMTarball(c *Ctx, tarball string) error {
	return step(
		func() error { return c.MkdirAll("/usr/local/gcm", 0o755) },
		func() error { return c.Run("tar", "-xzf", tarball, "-C", "/usr/local/gcm") },
		func() error {
			return c.Symlink("/usr/local/gcm/git-credential-manager", "/usr/local/bin/git-credential-manager")
		},
	)
}

const (
	fprGoogle        = "EB4C1BFD4F042F6DDDCCEC917721F63BD38B4796"
	googleChromeRepo = `[google-chrome]
name=google-chrome
baseurl=https://dl.google.com/linux/chrome/rpm/stable/x86_64
enabled=1
gpgcheck=1
gpgkey=https://dl.google.com/linux/linux_signing_key.pub
`
)

// instalarChromeRPM instala o Chrome pelo repositório do Google (o mesmo
// .repo que o próprio pacote cria), com a chave conferida antes: o dnf não
// verifica a assinatura de um .rpm local baixado à parte.
func instalarChromeRPM(c *Ctx) error {
	return step(
		func() error {
			return importarChaveRPM(c, "https://dl.google.com/linux/linux_signing_key.pub", "google.pub", fprGoogle)
		},
		func() error { return c.WriteFile("/etc/yum.repos.d/google-chrome.repo", googleChromeRepo, 0o644) },
		func() error { return c.install("google-chrome-stable") },
	)
}

var chromeGCM = Module{
	ID:    "chrome_gcm",
	Title: "Chrome + Git Credential Manager",
	Desc:  "Instala o Google Chrome e o Git Credential Manager",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando Git Credential Manager e Google Chrome...")
		const gcmRepo = "git-ecosystem/git-credential-manager"
		var g errgroup.Group

		switch c.Sys.PkgMgr {
		case "pacman", "dnf":
			asset, err := latestReleaseAsset(c, gcmRepo, gcmTarball)
			if err != nil {
				return err
			}
			gcm := c.Temp("gcm.tar.gz")
			// GCM e Chrome são independentes: baixa/instala os dois em paralelo.
			g.Go(func() error { return downloadAsset(c, asset, gcm) })
			if c.Sys.PkgMgr == "pacman" {
				g.Go(func() error { return c.installAUR("google-chrome") })
			} else {
				g.Go(func() error { return instalarChromeRPM(c) })
			}
			if err := g.Wait(); err != nil {
				return err
			}
			if err := instalarGCMTarball(c, gcm); err != nil {
				return err
			}
		default:
			asset, err := latestReleaseAsset(c, gcmRepo, gcmDeb)
			if err != nil {
				return err
			}
			gcm, chrome := c.Temp("gcm.deb"), c.Temp("chrome.deb")
			// Baixa os dois .deb em paralelo e instala ambos numa única chamada
			// do apt (evita duas resoluções de dependência). O .deb do Chrome não
			// é assinado individualmente, mas ao ser instalado ele configura o
			// repositório assinado do Google, por onde vêm as atualizações.
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
		var err error
		switch c.Sys.PkgMgr {
		case "pacman":
			err = c.installAUR("bitwarden-bin")
		default:
			variant := "deb"
			if c.Sys.PkgMgr == "dnf" {
				variant = "rpm"
			}
			pkg := c.Temp("bitwarden." + variant)
			err = step(
				func() error {
					return c.Download("https://vault.bitwarden.com/download/?app=desktop&platform=linux&variant="+variant, pkg)
				},
				func() error { return c.install(pkg) },
			)
		}
		if err != nil {
			return err
		}
		c.Log.Successf("Bitwarden instalado nativamente.")
		return nil
	},
}
