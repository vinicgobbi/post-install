package modules

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/vinicgobbi/post-install/internal/runner"
)

var rust = Module{
	ID:    "rust",
	Grupo: GrupoDesenvolvimento,
	Title: "Rust, eza, topgrade e cargo-update",
	Desc:  "rustup e compilação de eza, topgrade e cargo-update (cargo install-update, usado pelo topgrade para atualizar o que veio do cargo) via cargo",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando rustup e compilando eza/topgrade/cargo-update via cargo...")
		// O cargo-update compila com libgit2/libcurl/libssh2 (embutidos se
		// faltarem no sistema), o que exige CMake, pkg-config e os cabeçalhos
		// do OpenSSL.
		buildPkgs := []string{"build-essential", "cmake", "pkg-config", "libssl-dev"}
		err := step(
			func() error { return c.install(append(buildPkgs, "curl")...) },
			func() error {
				return c.AsUser(`set -e
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain stable
source "$HOME/.cargo/env"
cargo install --locked eza topgrade cargo-update
`)
			},
		)
		if err != nil {
			return err
		}
		c.Log.Successf("rustup, eza, topgrade e cargo-update instalados para %s.", c.User)
		return nil
	},
}

var gitGCM = Module{
	ID:    "git_gcm",
	Grupo: GrupoDesenvolvimento,
	Title: "Git Credential Manager",
	Desc:  "Instala o Git Credential Manager (última release, com SHA-256 conferido) como credential helper do Git, guardando as credenciais no chaveiro do GNOME",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando o Git Credential Manager...")
		asset, err := latestReleaseAsset(c, "git-ecosystem/git-credential-manager", gcmDeb)
		if err != nil {
			return err
		}
		gcm := c.Temp("gcm.deb")
		err = step(
			func() error { return downloadAsset(c, asset, gcm) },
			func() error { return c.install("git", gcm) },
			func() error {
				return c.AsUser(`git-credential-manager configure && git config --global credential.credentialStore secretservice`)
			},
		)
		if err != nil {
			return err
		}
		c.Log.Successf("Git Credential Manager instalado e configurado para %s.", c.User)
		return nil
	},
}

var docker = Module{
	ID:    "docker",
	Grupo: GrupoDesenvolvimento,
	Title: "Docker",
	Desc:  "Docker Engine, Buildx e Compose pelo repositório oficial; o usuário entra no grupo docker",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando o Docker pelo repositório oficial...")
		err := step(
			func() error {
				return instalarChaveAPT(c, "https://download.docker.com/linux/ubuntu/gpg", "/etc/apt/keyrings/docker.asc", fprDocker)
			},
			func() error {
				return c.WriteFile("/etc/apt/sources.list.d/docker.sources", "Types: deb\n"+
					"Architectures: "+c.Sys.Arch+"\n"+
					"URIs: https://download.docker.com/linux/ubuntu\n"+
					"Suites: "+c.Sys.Codename+"\n"+
					"Components: stable\n"+
					"Signed-By: /etc/apt/keyrings/docker.asc\n", 0o644)
			},
			func() error { return c.Run("apt", "update") },
			func() error {
				return c.install("docker-ce", "docker-ce-cli", "containerd.io", "docker-buildx-plugin", "docker-compose-plugin")
			},
			func() error { return c.Run("usermod", "-aG", "docker", c.User) },
		)
		if err != nil {
			return err
		}
		c.Log.Successf("Docker instalado. '%s' foi adicionado ao grupo docker (é preciso reabrir a sessão para valer).", c.User)
		return nil
	},
}

var vscode = Module{
	ID:    "vscode",
	Grupo: GrupoDesenvolvimento,
	Title: "Visual Studio Code",
	Desc:  "VSCode pelo repositório oficial da Microsoft",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando o VSCode pelo repositório da Microsoft...")
		asc := c.Temp("microsoft.asc")
		err := step(
			func() error { return c.Download("https://packages.microsoft.com/keys/microsoft.asc", asc) },
			func() error { return c.VerifyKey(asc, fprMicrosoft) },
			func() error { return c.MkdirAll("/etc/apt/keyrings", 0o755) },
			func() error {
				return c.Run("gpg", "--batch", "--yes", "--dearmor", "-o", "/etc/apt/keyrings/packages.microsoft.gpg", asc)
			},
			func() error {
				return c.WriteFile("/etc/apt/sources.list.d/vscode.list",
					"deb [arch=amd64,arm64,armhf signed-by=/etc/apt/keyrings/packages.microsoft.gpg] https://packages.microsoft.com/repos/code stable main\n", 0o644)
			},
			func() error { return c.Run("apt", "update") },
			func() error { return c.install("code") },
		)
		if err != nil {
			return err
		}
		c.Log.Successf("VSCode instalado.")
		return nil
	},
}

var golang = Module{
	ID:    "go",
	Grupo: GrupoDesenvolvimento,
	Title: "Go",
	Desc:  "Go pelo pacote golang do Ubuntu",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando o Go...")
		if err := c.install("golang"); err != nil {
			return err
		}
		c.Log.Successf("Go instalado.")
		return nil
	},
}

// scriptNode instala o Node LTS via fnm e põe a inicialização do fnm no
// .zshrc e no .bashrc (uma vez só). Roda depois do módulo shell, porque o
// instalador do Oh My Zsh troca o ~/.zshrc.
const scriptNode = `set -e
export PATH="$HOME/.local/share/fnm:$PATH"
eval "$(fnm env --shell bash)"
fnm install --lts
fnm default $(fnm current)

for rc in "$HOME/.zshrc" "$HOME/.bashrc"; do
    [ -f "$rc" ] || continue
    sh=$(basename "$rc" rc | tr -d .)
    grep -qF 'fnm env' "$rc" || printf '\n# fnm (Node) — post-install\nexport PATH="$HOME/.local/share/fnm:$PATH"\neval "$(fnm env --shell %s)"\n' "$sh" >> "$rc"
done
`

var node = Module{
	ID:    "node",
	Grupo: GrupoDesenvolvimento,
	Title: "Node.js (fnm)",
	Desc:  "fnm e o Node LTS para o usuário, com o fnm iniciado no .zshrc e no .bashrc",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando o fnm e o Node LTS para %s...", c.User)
		err := step(
			func() error { return c.install("curl", "unzip") },
			func() error {
				return c.AsUser(`curl -fsSL https://fnm.vercel.app/install | bash -s -- --skip-shell`)
			},
			func() error { return c.AsUser(scriptNode) },
		)
		if err != nil {
			return err
		}
		c.Log.Successf("fnm e Node LTS instalados para %s.", c.User)
		return nil
	},
}

var phpSQLServer = Module{
	ID:    "php_sqlserver",
	Grupo: GrupoDesenvolvimento,
	Title: "PHP e SQL Server",
	Desc:  "PHP, Composer, as ferramentas do SQL Server (msodbcsql18 e mssql-tools18, pelo repositório da Microsoft) e as extensões sqlsrv/pdo_sqlsrv via PECL",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando PHP, Composer e as ferramentas do SQL Server...")
		deb := c.Temp("packages-microsoft-prod.deb")
		err := step(
			func() error {
				return c.Download("https://packages.microsoft.com/config/ubuntu/"+c.Sys.MSRepoVersion+"/packages-microsoft-prod.deb", deb)
			},
			func() error { return c.Run("dpkg", "-i", deb) },
			func() error { return c.Run("apt", "update") },
			func() error { return c.install("php", "composer", "php-dev", "php-xml", "php-pear") },
			func() error {
				return c.installEnv([]string{"ACCEPT_EULA=Y"}, "msodbcsql18", "mssql-tools18", "unixodbc-dev")
			},
			func() error {
				return c.WriteFile("/etc/profile.d/mssql-tools.sh", `export PATH="$PATH:/opt/mssql-tools18/bin"`+"\n", 0o644)
			},
		)
		if err != nil {
			return err
		}

		c.Log.Infof("Compilando e habilitando as extensões sqlsrv/pdo_sqlsrv...")
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
		if err != nil {
			return err
		}
		c.Log.Successf("PHP, Composer, ferramentas e extensões do SQL Server instalados.")
		return nil
	},
}

// O Bundler e o Jekyll são instalados como gems do usuário (GEM_HOME na
// home), como recomenda a documentação do Jekyll: assim gem/bundle nunca
// pedem sudo, e as versões não ficam presas às dos pacotes da distro.
const rubyAmbiente = `# Gems do Ruby na home do usuário (gem/bundle sem sudo) — post-install
export GEM_HOME="$HOME/.local/share/gem"
export PATH="$GEM_HOME/bin:$PATH"
`

// scriptRuby instala as gems na home do usuário, configura o Bundler e
// acrescenta GEM_HOME/PATH ao .zshrc e ao .bashrc (uma vez só).
func scriptRuby() string {
	return `set -e
` + rubyAmbiente + `
gem install --no-document bundler jekyll
bundle config set --global path vendor/bundle

for rc in "$HOME/.zshrc" "$HOME/.bashrc"; do
    [ -f "$rc" ] || continue
    grep -qF 'GEM_HOME=' "$rc" || printf '\n%s' ` + shQuote(rubyAmbiente) + ` >> "$rc"
done
`
}

var ruby = Module{
	ID:    "ruby",
	Grupo: GrupoDesenvolvimento,
	Title: "Ruby, Bundler e Jekyll",
	Desc:  "Ruby com headers de desenvolvimento e, como gems do usuário (sem sudo), Bundler e Jekyll; o Bundler instala as gems de cada projeto em vendor/bundle",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando Ruby e compiladores para gems nativas...")
		pkgs := []string{"ruby-full", "ruby-dev", "build-essential", "zlib1g-dev"}
		if err := c.install(pkgs...); err != nil {
			return err
		}

		c.Log.Infof("Instalando Bundler e Jekyll como gems de %s...", c.User)
		// "path vendor/bundle" (global, em ~/.bundle/config): o bundle install
		// de cada projeto grava as gems dentro do próprio projeto. O Jekyll já
		// ignora vendor/ no build, e o .gitignore do "jekyll new" também.
		if err := c.AsUser(scriptRuby()); err != nil {
			return err
		}
		c.Log.Successf("Ruby, Bundler e Jekyll instalados para %s (gems em ~/.local/share/gem; projetos em vendor/bundle).", c.User)
		return nil
	},
}

var claudeCode = Module{
	ID:    "claude_code",
	Grupo: GrupoDesenvolvimento,
	Title: "Claude Code",
	Desc:  "CLI oficial da Anthropic para desenvolvimento assistido por IA no terminal",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando o Claude Code...")
		if err := c.garantir("curl", "curl"); err != nil {
			return err
		}
		if err := c.AsUser(`curl -fsSL https://claude.ai/install.sh | bash`); err != nil {
			return err
		}
		c.Log.Successf("Claude Code instalado para %s.", c.User)
		return nil
	},
}

// O deploy-dev se instala sozinho (install copia para ~/.local/bin e põe a
// pasta no PATH) e depois se atualiza sozinho a partir das releases.
var deployDev = Module{
	ID:    "deploy_dev",
	Grupo: GrupoDesenvolvimento,
	Title: "deploy-dev",
	Desc:  "CLI do deploy-tools que gera e valida o workflow de deploy dos projetos (github.com/vinicgobbi/deploy-tools)",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando o deploy-dev (última release do deploy-tools)...")
		const repo = "vinicgobbi/deploy-tools"
		bin, err := latestReleaseAsset(c, repo, func(n string) bool { return n == "deploy-dev-linux-amd64" })
		if err != nil {
			return err
		}
		sum, err := latestReleaseAsset(c, repo, func(n string) bool { return n == "deploy-dev-linux-amd64.sha256" })
		if err != nil {
			return err
		}
		dest := c.Temp("deploy-dev-linux-amd64")
		if err := downloadAsset(c, bin, dest); err != nil {
			return err
		}
		if !c.DryRun {
			if err := conferirSHA256(c, sum.URL, dest); err != nil {
				return err
			}
		}
		// O binário baixado é do root; o usuário copia para um temporário
		// dele para poder marcar como executável.
		if err := c.AsUser(`set -e
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
install -m 0755 ` + shQuote(dest) + ` "$dir/deploy-dev"
"$dir/deploy-dev" install --add-to-path
`); err != nil {
			return err
		}
		c.Log.Successf("deploy-dev instalado em ~/.local/bin para %s.", c.User)
		return nil
	},
}

// conferirSHA256 compara o arquivo com o .sha256 publicado na release
// (formato do sha256sum: "<hash>  <nome>").
func conferirSHA256(c *Ctx, sumURL, arquivo string) error {
	body, err := c.Get(sumURL, 30*time.Second)
	if err != nil {
		return err
	}
	campos := strings.Fields(string(body))
	if len(campos) == 0 {
		return errorf("arquivo de SHA-256 vazio: %s", sumURL)
	}
	return runner.VerifySHA256(arquivo, campos[0])
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
