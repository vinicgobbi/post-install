package modules

import (
	"os"
	"regexp"

	"github.com/vinicgobbi/post-install/internal/runner"
)

var atualizacao = Module{
	ID:    "atualizacao",
	Title: "Atualizar sistema",
	Desc:  "Roda o upgrade completo de pacotes do sistema",
	Run: func(c *Ctx) error {
		c.Log.Infof("Atualizando pacotes do sistema...")
		var err error
		switch c.Sys.PkgMgr {
		case "dnf":
			err = c.Run("dnf", "upgrade", "--refresh", "-y")
		case "pacman":
			err = c.Run("pacman", "-Syu", "--noconfirm")
		default:
			err = step(
				func() error { return c.Run("apt", "update") },
				func() error { return c.Run("apt", "upgrade", "-y") },
			)
		}
		if err != nil {
			return err
		}
		c.Log.Successf("Sistema atualizado.")
		return nil
	},
}

// Chave GPG oficial do Chaotic-AUR (aur.chaotic.cx) — repo binário que serve
// pacotes pré-compilados da AUR (yay incluso), evitando compilar tudo na mão.
const (
	chaoticAURKey  = "3056513887B78AEB"
	sudoersAURFile = "/etc/sudoers.d/99-post-install-aur"
)

var repositorios = Module{
	ID:    "repositorios",
	Title: "Configurar repositórios",
	Desc:  "Docker/VSCode/MS SQL/GitHub CLI e Mozilla no Ubuntu (Fedora/RHEL/Debian) ou Chaotic-AUR + yay (Arch)",
	Run: func(c *Ctx) error {
		c.Log.Infof("Configurando repositórios (Docker, Microsoft, GitHub CLI)...")
		var err error
		switch c.Sys.PkgMgr {
		case "pacman":
			err = step(
				func() error {
					return c.install("curl", "gnupg", "jq", "base-devel", "git", "reflector")
				},
				func() error { return configurarChaoticAUR(c) },
				func() error { return permitirSudoPacmanTemporario(c) },
				func() error { return instalarYay(c) },
			)
		case "dnf":
			err = repositoriosDNF(c)
		default:
			err = repositoriosAPT(c)
		}
		if err != nil {
			return err
		}
		c.Log.Successf("Repositórios configurados.")
		return nil
	},
}

var reChaoticSection = regexp.MustCompile(`(?m)^\[chaotic-aur\]`)

// configurarChaoticAUR segue o processo oficial (https://aur.chaotic.cx/):
// importa a chave, instala keyring+mirrorlist via pacman -U e inclui o
// mirrorlist no pacman.conf. Idempotente.
func configurarChaoticAUR(c *Ctx) error {
	if conf, err := os.ReadFile("/etc/pacman.conf"); err == nil && reChaoticSection.Match(conf) {
		c.Log.Infof("Chaotic-AUR já configurado.")
		return nil
	}
	c.Log.Infof("Configurando o repositório binário Chaotic-AUR...")
	err := step(
		func() error {
			return c.Run("pacman-key", "--recv-key", chaoticAURKey, "--keyserver", "keyserver.ubuntu.com")
		},
		func() error { return c.Run("pacman-key", "--lsign-key", chaoticAURKey) },
		func() error {
			return c.Run("pacman", "-U", "--noconfirm",
				"https://cdn-mirror.chaotic.cx/chaotic-aur/chaotic-keyring.pkg.tar.zst",
				"https://cdn-mirror.chaotic.cx/chaotic-aur/chaotic-mirrorlist.pkg.tar.zst")
		},
		func() error {
			return c.AppendFile("/etc/pacman.conf", "\n[chaotic-aur]\nInclude = /etc/pacman.d/chaotic-mirrorlist\n")
		},
		func() error { return c.Run("pacman", "-Sy") },
	)
	if err != nil {
		return err
	}
	c.Log.Successf("Chaotic-AUR configurado.")
	return nil
}

// O yay (e o makepkg por baixo) recusa rodar como root, mas este programa
// roda via sudo. Sem essa liberação, todo módulo que instala algo da AUR
// pararia pedindo senha. A regra é restrita (só o binário do pacman) e
// temporária: removida pelo módulo "limpeza".
func permitirSudoPacmanTemporario(c *Ctx) error {
	if _, err := os.Stat(sudoersAURFile); err == nil {
		return nil
	}
	c.Log.Infof("Liberando sudo sem senha para o pacman (usuário %s) — necessário para o yay instalar pacotes da AUR sem interação; revertido ao final em 'Limpeza final'.", c.User)
	if err := c.WriteFile(sudoersAURFile, c.User+" ALL=(ALL) NOPASSWD: /usr/bin/pacman\n", 0o440); err != nil {
		return err
	}
	if err := c.Run("visudo", "-cf", sudoersAURFile); err != nil {
		c.RemoveAll(sudoersAURFile)
		return errorf("falha ao validar a regra temporária de sudo para o pacman: %w", err)
	}
	return nil
}

// instalarYay tenta primeiro o binário pronto do Chaotic-AUR; se não estiver
// disponível, compila da AUR como o usuário alvo.
func instalarYay(c *Ctx) error {
	if runner.Exists("yay") {
		return nil
	}
	if c.Try("pacman", "-S", "--needed", "--noconfirm", "yay") {
		c.Log.Successf("yay instalado via Chaotic-AUR.")
		return nil
	}
	c.Log.Infof("yay não encontrado como binário no Chaotic-AUR; compilando da AUR...")
	err := c.AsUser(`
rm -rf /tmp/yay-build
git clone https://aur.archlinux.org/yay.git /tmp/yay-build
cd /tmp/yay-build
makepkg -si --noconfirm
`)
	c.RemoveAll("/tmp/yay-build")
	if err != nil {
		return err
	}
	c.Log.Successf("yay compilado e instalado.")
	return nil
}

const vscodeYumRepo = `[code]
name=Visual Studio Code
baseurl=https://packages.microsoft.com/yumrepos/vscode
enabled=1
gpgcheck=1
gpgkey=https://packages.microsoft.com/keys/microsoft.asc
`

func repositoriosDNF(c *Ctx) error {
	if err := c.install("curl", "gnupg2", "jq", "tar", "gcc", "gcc-c++", "make", "dnf-plugins-core"); err != nil {
		return err
	}

	dockerRepo := "https://download.docker.com/linux/fedora/docker-ce.repo"
	// A Microsoft não publica repositório "prod" próprio para Fedora; o do
	// RHEL 9 é o substituto oficialmente indicado por eles.
	msRepo := "https://packages.microsoft.com/config/rhel/9/prod.repo"
	if c.Sys.Family == "rhel" {
		if err := c.install("epel-release"); err != nil {
			return err
		}
		if err := c.Run("dnf", "config-manager", "--set-enabled", "crb"); err != nil {
			c.Log.Warnf("Não foi possível habilitar o repositório CRB: %v", err)
		}
		dockerRepo = "https://download.docker.com/linux/centos/docker-ce.repo"
		msRepo = "https://packages.microsoft.com/config/rhel/" + c.Sys.RHELVersion + "/prod.repo"
	}

	return step(
		func() error { return c.Download(dockerRepo, "/etc/yum.repos.d/docker-ce.repo") },
		func() error { return c.Download(msRepo, "/etc/yum.repos.d/msprod.repo") },
		func() error { return c.Run("rpm", "--import", "https://packages.microsoft.com/keys/microsoft.asc") },
		func() error { return c.WriteFile("/etc/yum.repos.d/vscode.repo", vscodeYumRepo, 0o644) },
		func() error {
			return c.Download("https://cli.github.com/packages/rpm/gh-cli.repo", "/etc/yum.repos.d/gh-cli.repo")
		},
	)
}

func repositoriosAPT(c *Ctx) error {
	s := c.Sys
	err := step(
		func() error { return c.MkdirAll("/etc/apt/keyrings", 0o755) },
		func() error { return c.install("ca-certificates", "curl", "gnupg", "ufw", "gufw") },
		func() error {
			return c.Download("https://download.docker.com/linux/"+s.DockerDistro+"/gpg", "/etc/apt/keyrings/docker.asc")
		},
		func() error { return c.Chmod("/etc/apt/keyrings/docker.asc", 0o644) },
		func() error {
			return c.WriteFile("/etc/apt/sources.list.d/docker.sources", "Types: deb\n"+
				"Architectures: "+s.Arch+"\n"+
				"URIs: https://download.docker.com/linux/"+s.DockerDistro+"\n"+
				"Suites: "+s.BaseCodename+"\n"+
				"Components: stable\n"+
				"Signed-By: /etc/apt/keyrings/docker.asc\n", 0o644)
		},
	)
	if err != nil {
		return err
	}

	if s.MSRepoSupported {
		deb := c.Temp("packages-microsoft-prod.deb")
		err := step(
			func() error {
				return c.Download("https://packages.microsoft.com/config/"+s.DockerDistro+"/"+s.DistroVersion+"/packages-microsoft-prod.deb", deb)
			},
			func() error { return c.Run("dpkg", "-i", deb) },
		)
		if err != nil {
			return err
		}
	} else {
		c.Log.Warnf("Sem repositório oficial da Microsoft para %s %s: msodbcsql/mssql-tools serão pulados mais adiante.", s.DockerDistro, s.DistroVersion)
	}

	if usaRepoMozilla(c) {
		if err := configurarRepoMozilla(c); err != nil {
			return err
		}
	}

	// GitHub CLI pelo repositório oficial (https://github.com/cli/cli/blob/trunk/docs/install_linux.md):
	// o "gh" dos repositórios da distro fica bem atrás das releases.
	err = step(
		func() error {
			return c.Download("https://cli.github.com/packages/githubcli-archive-keyring.gpg", "/etc/apt/keyrings/githubcli-archive-keyring.gpg")
		},
		func() error { return c.Chmod("/etc/apt/keyrings/githubcli-archive-keyring.gpg", 0o644) },
		func() error {
			return c.WriteFile("/etc/apt/sources.list.d/github-cli.list",
				"deb [arch="+s.Arch+" signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main\n", 0o644)
		},
	)
	if err != nil {
		return err
	}

	asc := c.Temp("microsoft.asc")
	return step(
		func() error { return c.Download("https://packages.microsoft.com/keys/microsoft.asc", asc) },
		func() error {
			return c.Run("gpg", "--batch", "--yes", "--dearmor", "-o", "/etc/apt/keyrings/packages.microsoft.gpg", asc)
		},
		func() error {
			return c.WriteFile("/etc/apt/sources.list.d/vscode.list",
				"deb [arch=amd64,arm64,armhf signed-by=/etc/apt/keyrings/packages.microsoft.gpg] https://packages.microsoft.com/repos/code stable main\n", 0o644)
		},
		func() error { return c.Run("apt", "update") },
	)
}

// ferramentas devolve os utilitários de terminal com os nomes de pacote de
// cada distro. O FUSE 2 é para AppImages: no Ubuntu 24.04+ e Debian 13 o
// pacote ganhou o sufixo "t64" (transição do time_t de 64 bits).
func ferramentas(c *Ctx) []string {
	switch c.Sys.PkgMgr {
	case "dnf":
		return []string{"gh", "nmap", "pipx", "ffmpeg-free", "bashtop", "fuse-libs"}
	case "pacman":
		return []string{"github-cli", "nmap", "python-pipx", "ffmpeg", "bashtop", "fuse2"}
	default:
		fuse := "libfuse2t64"
		if !c.Probe("apt-cache", "show", fuse) {
			fuse = "libfuse2"
		}
		return []string{"gh", "nmap", "pipx", "ffmpeg", "bashtop", fuse}
	}
}

// instalarFerramentas tenta tudo numa chamada só; se falhar (algum pacote
// ausente na distro), instala um por um e só avisa dos que faltarem, sem
// derrubar o módulo.
func instalarFerramentas(c *Ctx) {
	c.Log.Infof("Instalando ferramentas de terminal (gh, nmap, pipx, ffmpeg, bashtop, FUSE 2)...")
	pkgs := ferramentas(c)
	if c.install(pkgs...) == nil {
		return
	}
	for _, p := range pkgs {
		if err := c.install(p); err != nil {
			c.Log.Warnf("Não foi possível instalar %s: %v", p, err)
		}
	}
}

var pacotesBase = Module{
	ID:    "pacotes_base",
	Title: "Instalar pacotes base",
	Desc:  "Docker, VSCode, PHP, Zsh, Firefox, Thunderbird, ferramentas de SQL Server e de terminal (gh, nmap, pipx, ffmpeg, bashtop, FUSE 2 para AppImages; no Ubuntu, Firefox/Thunderbird em .deb da Mozilla, removendo os snaps)",
	Deps:  []string{"repositorios"},
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando pacotes base e utilitários...")
		eula := []string{"ACCEPT_EULA=Y"}
		var err error
		switch c.Sys.PkgMgr {
		case "pacman":
			// Docker, PHP/composer e unixODBC são oficiais; code, msodbcsql e
			// mssql-tools só existem na AUR (o yay resolve pelo Chaotic-AUR
			// quando disponível, sem compilar).
			err = step(
				func() error {
					return c.install("flatpak", "zsh", "git", "curl", "jq", "xdg-user-dirs",
						"docker", "docker-compose", "docker-buildx", "php", "composer", "unixodbc", "bat")
				},
				func() error { return c.installAUR("visual-studio-code-bin", "msodbcsql", "mssql-tools") },
			)
		case "dnf":
			err = c.installEnv(eula, "flatpak", "code", "zsh", "git", "curl", "jq",
				"docker-ce", "docker-ce-cli", "containerd.io", "docker-buildx-plugin", "docker-compose-plugin", "php", "composer",
				"php-devel", "php-xml", "php-pear", "msodbcsql18", "mssql-tools18", "unixODBC-devel", "bat")
		default:
			extras := []string{"code"}
			// gnome-software só faz sentido em Ubuntu com GNOME; no Kubuntu o
			// Discover já cobre Flatpak/Snap.
			if c.Sys.ID == "ubuntu" && !c.Sys.IsKDE() {
				extras = append(extras, "gnome-software", "gnome-software-plugin-flatpak", "gnome-software-plugin-snap")
			}
			pkgs := append([]string{"flatpak", "zsh", "git", "curl", "jq",
				"docker-ce", "docker-ce-cli", "containerd.io", "docker-buildx-plugin", "docker-compose-plugin", "php", "composer",
				"php-dev", "php-xml", "php-pear", "bat"}, extras...)
			err = c.install(pkgs...)
			if err == nil {
				if c.Sys.MSRepoSupported {
					err = c.installEnv(eula, "msodbcsql18", "mssql-tools18", "unixodbc-dev")
				} else {
					c.Log.Warnf("Pulando msodbcsql18/mssql-tools18 (sem repositório oficial da Microsoft para esta distro/versão).")
				}
			}
		}
		if err != nil {
			return err
		}
		if err := instalarFirefoxThunderbird(c); err != nil {
			return err
		}
		instalarFerramentas(c)

		// No Debian/Ubuntu o "bat" instala o binário como "batcat" por
		// conflito de nome; cria um link para "bat" funcionar em todo lugar.
		if !runner.Exists("bat") {
			if batcat, err := lookPath("batcat"); err == nil {
				if err := c.Symlink(batcat, "/usr/local/bin/bat"); err != nil {
					return err
				}
			}
		}

		// Debian/Fedora instalam em /opt/mssql-tools18; o pacote da AUR em
		// /opt/mssql-tools — inclui os dois no PATH.
		if err := c.WriteFile("/etc/profile.d/mssql-tools.sh",
			`export PATH="$PATH:/opt/mssql-tools18/bin:/opt/mssql-tools/bin"`+"\n", 0o644); err != nil {
			return err
		}
		c.Log.Successf("Pacotes e MS SQL instalados.")
		return nil
	},
}
