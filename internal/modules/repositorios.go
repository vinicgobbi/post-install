package modules

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/vinicgobbi/post-install/internal/runner"
)

var atualizacao = Module{
	ID:    "atualizacao",
	Title: "Atualizar sistema",
	Desc:  "Roda o upgrade completo de pacotes do sistema",
	Run: func(c *Ctx) error {
		c.Log.Infof("Atualizando pacotes do sistema...")
		err := step(
			func() error { return c.Run("apt", "update") },
			func() error { return c.Run("apt", "upgrade", "-y") },
		)
		if err != nil {
			return err
		}
		c.Log.Successf("Sistema atualizado.")
		return nil
	},
}

// Fingerprints das chaves primárias que assinam cada repositório. As chaves
// são baixadas dos sites oficiais e conferidas contra estes valores antes de
// serem instaladas, para um download adulterado não virar repositório
// confiável do sistema.
const (
	fprDocker    = "9DC858229FC7DD38854AE2D88D81803C0EBFCD88"
	fprMicrosoft = "BC528686B50D79E339D3721CEB3E94ADBE1229CF"
	// O keyring do GitHub CLI traz a chave atual e a anterior (expirada).
	fprGitHubCLI    = "7F38BBB59D064DBCB3D84D725612B36462313325"
	fprGitHubCLIOld = "2C6106201985B60E6C7AC87323F3D4EA75716059"
)

// instalarChaveAPT baixa uma chave para /etc/apt/keyrings só depois de
// conferir o fingerprint.
func instalarChaveAPT(c *Ctx, url, destino string, fprs ...string) error {
	tmp := c.Temp(filepath.Base(destino))
	return step(
		func() error { return c.Download(url, tmp) },
		func() error { return c.VerifyKey(tmp, fprs...) },
		func() error { return c.Run("install", "-D", "-m", "0644", tmp, destino) },
	)
}

var repositorios = Module{
	ID:    "repositorios",
	Title: "Configurar repositórios",
	Desc:  "Docker, VSCode, MS SQL, GitHub CLI e Mozilla (Firefox/Thunderbird sem snap)",
	Run: func(c *Ctx) error {
		c.Log.Infof("Configurando repositórios (Docker, Microsoft, GitHub CLI, Mozilla)...")
		if err := repositoriosAPT(c); err != nil {
			return err
		}
		c.Log.Successf("Repositórios configurados.")
		return nil
	},
}

func repositoriosAPT(c *Ctx) error {
	s := c.Sys
	err := step(
		func() error { return c.MkdirAll("/etc/apt/keyrings", 0o755) },
		func() error { return c.install("ca-certificates", "curl", "gnupg", "ufw", "gufw") },
		func() error {
			return instalarChaveAPT(c, "https://download.docker.com/linux/ubuntu/gpg", "/etc/apt/keyrings/docker.asc", fprDocker)
		},
		func() error {
			return c.WriteFile("/etc/apt/sources.list.d/docker.sources", "Types: deb\n"+
				"Architectures: "+s.Arch+"\n"+
				"URIs: https://download.docker.com/linux/ubuntu\n"+
				"Suites: "+s.Codename+"\n"+
				"Components: stable\n"+
				"Signed-By: /etc/apt/keyrings/docker.asc\n", 0o644)
		},
	)
	if err != nil {
		return err
	}

	deb := c.Temp("packages-microsoft-prod.deb")
	err = step(
		func() error {
			return c.Download("https://packages.microsoft.com/config/ubuntu/"+s.MSRepoVersion+"/packages-microsoft-prod.deb", deb)
		},
		func() error { return c.Run("dpkg", "-i", deb) },
		func() error { return configurarRepoMozilla(c) },
	)
	if err != nil {
		return err
	}

	// GitHub CLI pelo repositório oficial (https://github.com/cli/cli/blob/trunk/docs/install_linux.md):
	// o "gh" dos repositórios da distro fica bem atrás das releases.
	err = step(
		func() error {
			return instalarChaveAPT(c, "https://cli.github.com/packages/githubcli-archive-keyring.gpg",
				"/etc/apt/keyrings/githubcli-archive-keyring.gpg", fprGitHubCLI, fprGitHubCLIOld)
		},
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
		func() error { return c.VerifyKey(asc, fprMicrosoft) },
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

// ferramentas devolve os utilitários de terminal. O FUSE 2 é para AppImages:
// no Ubuntu 24.04+ o pacote ganhou o sufixo "t64" (transição do time_t de 64
// bits).
func ferramentas(c *Ctx) []string {
	fuse := "libfuse2t64"
	if !c.Probe("apt-cache", "show", fuse) {
		fuse = "libfuse2"
	}
	return []string{"gh", "nmap", "pipx", "ffmpeg", "bashtop", fuse}
}

// instalarFerramentas tenta tudo numa chamada só; se falhar (algum pacote
// ausente na versão), instala um por um e só avisa dos que faltarem, sem
// derrubar o módulo.
func instalarFerramentas(c *Ctx) {
	c.Log.Infof("Instalando ferramentas de terminal (gh, tea, nmap, pipx, ffmpeg, bashtop, FUSE 2)...")
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

// instalarTea instala o Gitea CLI pelo binário da última release em
// gitea.com, conferindo o SHA-256 publicado: o Ubuntu não tem pacote do tea.
func instalarTea(c *Ctx) error {
	body, err := c.Get("https://gitea.com/api/v1/repos/gitea/tea/releases/latest", 30*time.Second)
	if err != nil {
		return err
	}
	var rel struct {
		Tag string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return err
	}
	// Só o .xz tem .sha256 publicado (o binário solto não).
	nome := "tea-" + strings.TrimPrefix(rel.Tag, "v") + "-linux-" + runtime.GOARCH + ".xz"
	base := "https://gitea.com/gitea/tea/releases/download/" + rel.Tag + "/"
	sum, err := c.Get(base+nome+".sha256", 30*time.Second)
	if err != nil {
		return err
	}
	campos := strings.Fields(string(sum))
	if len(campos) == 0 {
		return errorf("arquivo de SHA-256 vazio: %s", base+nome+".sha256")
	}
	xz := c.Temp(nome)
	return step(
		func() error { return c.DownloadVerified(base+nome, xz, campos[0]) },
		func() error { return c.Run("xz", "-df", xz) },
		func() error {
			return c.Run("install", "-m", "0755", strings.TrimSuffix(xz, ".xz"), "/usr/local/bin/tea")
		},
	)
}

var pacotesBase = Module{
	ID:    "pacotes_base",
	Title: "Instalar pacotes base",
	Desc:  "Docker, VSCode, PHP, Go, Zsh, Firefox/Thunderbird (.deb da Mozilla, removendo os snaps), ferramentas de SQL Server e de terminal (gh, tea, nmap, pipx, ffmpeg, bashtop, FUSE 2 para AppImages)",
	Deps:  []string{"repositorios"},
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando pacotes base e utilitários...")
		err := step(
			func() error {
				return c.install("flatpak", "zsh", "git", "curl", "jq",
					"docker-ce", "docker-ce-cli", "containerd.io", "docker-buildx-plugin", "docker-compose-plugin", "php", "composer",
					"php-dev", "php-xml", "php-pear", "bat", "code", "golang",
					"gnome-software", "gnome-software-plugin-flatpak", "gnome-software-plugin-snap")
			},
			func() error {
				return c.installEnv([]string{"ACCEPT_EULA=Y"}, "msodbcsql18", "mssql-tools18", "unixodbc-dev")
			},
			func() error { return instalarFirefoxThunderbird(c) },
		)
		if err != nil {
			return err
		}
		instalarFerramentas(c)
		c.Log.Infof("Instalando o Gitea CLI (tea)...")
		if err := instalarTea(c); err != nil {
			c.Log.Warnf("Não foi possível instalar o tea: %v", err)
		}

		// No Ubuntu o "bat" instala o binário como "batcat" por conflito de
		// nome; cria um link para "bat" funcionar em todo lugar.
		if !runner.Exists("bat") {
			if batcat, err := lookPath("batcat"); err == nil {
				if err := c.Symlink(batcat, "/usr/local/bin/bat"); err != nil {
					return err
				}
			}
		}

		if err := c.WriteFile("/etc/profile.d/mssql-tools.sh",
			`export PATH="$PATH:/opt/mssql-tools18/bin"`+"\n", 0o644); err != nil {
			return err
		}
		c.Log.Successf("Pacotes e MS SQL instalados.")
		return nil
	},
}
