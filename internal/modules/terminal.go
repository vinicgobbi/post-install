package modules

import (
	"encoding/json"
	"runtime"
	"strings"
	"time"

	"github.com/vinicgobbi/post-install/internal/runner"
)

// pacotesFerramentas devolve os utilitários de terminal. O FUSE 2 é para AppImages:
// no Ubuntu 24.04+ o pacote ganhou o sufixo "t64" (transição do time_t de 64
// bits).
func pacotesFerramentas(c *Ctx) []string {
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
	pkgs := pacotesFerramentas(c)
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

// configurarRepoGitHubCLI segue https://github.com/cli/cli/blob/trunk/docs/install_linux.md:
// o "gh" dos repositórios do Ubuntu fica bem atrás das releases.
func configurarRepoGitHubCLI(c *Ctx) error {
	return step(
		func() error {
			return instalarChaveAPT(c, "https://cli.github.com/packages/githubcli-archive-keyring.gpg",
				"/etc/apt/keyrings/githubcli-archive-keyring.gpg", fprGitHubCLI, fprGitHubCLIOld)
		},
		func() error {
			return c.WriteFile("/etc/apt/sources.list.d/github-cli.list",
				"deb [arch="+c.Sys.Arch+" signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main\n", 0o644)
		},
		func() error { return c.Run("apt", "update") },
	)
}

var ferramentas = Module{
	ID:    "ferramentas",
	Grupo: GrupoTerminal,
	Title: "Ferramentas de terminal",
	Desc:  "git, curl, jq, bat, gh (repositório oficial), tea, nmap, pipx, ffmpeg, bashtop e FUSE 2 para AppImages",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando ferramentas de terminal...")
		err := step(
			func() error { return c.install("git", "curl", "jq", "bat") },
			func() error { return configurarRepoGitHubCLI(c) },
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
		c.Log.Successf("Ferramentas de terminal instaladas.")
		return nil
	},
}

var shell = Module{
	ID:    "shell",
	Grupo: GrupoTerminal,
	Title: "Zsh e dotfiles",
	Desc:  "Zsh como shell padrão e os dotfiles pessoais (Oh My Zsh, plugins, tema, fontes e configurações)",
	Run: func(c *Ctx) error {
		c.Log.Infof("Configurando o Zsh e os dotfiles para %s...", c.User)
		if err := c.install("zsh", "git", "curl"); err != nil {
			return err
		}

		// chsh só aceita contas presentes de fato em /etc/passwd; contas de
		// domínio (AD/LDAP via sssd/winbind) resolvem por NSS, mas não estão
		// no arquivo. A resolução restrita à fonte "files" distingue os casos.
		if c.Probe("getent", "-s", "files", "passwd", c.User) {
			zsh, err := lookPath("zsh")
			if err != nil {
				return errorf("zsh não encontrado: %w", err)
			}
			if err := c.Run("chsh", "-s", zsh, c.User); err != nil {
				return err
			}
		} else {
			c.Log.Warnf("'%s' é uma conta de domínio (AD/LDAP), não local: pulando 'chsh' (não dá para trocar o shell padrão por aqui nesse caso).", c.User)
		}

		if err := c.AsUser("set -e\ndir=$(mktemp -d)\ntrap 'rm -rf \"$dir\"' EXIT\n" +
			clonarFixado(dotfilesRepo, dotfilesCommit, `"$dir"`) +
			`bash "$dir/bootstrap.sh"` + "\n"); err != nil {
			return err
		}
		c.Log.Successf("Zsh e dotfiles configurados para %s.", c.User)
		return nil
	},
}
