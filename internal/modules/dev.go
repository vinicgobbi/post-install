package modules

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vinicgobbi/post-install/internal/runner"
)

var rust = Module{
	ID:    "rust",
	Grupo: GrupoDesenvolvimento,
	Title: "Rust, eza, topgrade e cargo-update",
	Desc:  "Rust stable pelo mise e compilação de eza, topgrade e cargo-update (cargo install-update, usado pelo topgrade para atualizar o que veio do cargo) via cargo",
	Deps:  []string{"mise"},
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando o Rust pelo mise e compilando eza/topgrade/cargo-update via cargo...")
		// O cargo-update compila com libgit2/libcurl/libssh2 (embutidos se
		// faltarem no sistema), o que exige CMake, pkg-config e os cabeçalhos
		// do OpenSSL.
		buildPkgs := []string{"build-essential", "cmake", "pkg-config", "libssl-dev"}
		err := step(
			func() error { return c.install(buildPkgs...) },
			func() error {
				return c.AsUser(`set -e
mise use -g rust@stable
mise exec -- cargo install --locked eza topgrade cargo-update
`)
			},
		)
		if err != nil {
			return err
		}
		c.Log.Successf("Rust (mise), eza, topgrade e cargo-update instalados para %s.", c.User)
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

// O Podman roda sem daemon e sem root: não há grupo com poder de root como o
// "docker", mas o usuário precisa de uma faixa de UIDs/GIDs subordinados
// (/etc/subuid e /etc/subgid) para o modo rootless.
var podman = Module{
	ID:    "podman",
	Grupo: GrupoDesenvolvimento,
	Title: "Podman",
	Desc:  "Podman e podman-compose pelos repositórios do Ubuntu (sem daemon e sem root); o usuário recebe faixas de subuid/subgid se ainda não tiver, e imagens sem registry (ex.: \"nginx\") vêm do Docker Hub",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando o Podman e o podman-compose...")
		err := step(
			func() error { return c.install("podman", "podman-compose", "uidmap") },
			func() error { return garantirSubIDs(c, "/etc/subuid") },
			func() error { return garantirSubIDs(c, "/etc/subgid") },
			func() error { return c.MkdirAll("/etc/containers/registries.conf.d", 0o755) },
			func() error {
				return c.WriteFile("/etc/containers/registries.conf.d/50-post-install.conf",
					"# Nomes curtos de imagem (ex.: \"nginx\", comuns em compose.yaml) vêm do Docker Hub — post-install\n"+
						"unqualified-search-registries = [\"docker.io\"]\n", 0o644)
			},
		)
		if err != nil {
			return err
		}
		// Aplica as faixas novas a um armazenamento que já exista.
		if err := c.AsUser(`podman system migrate`); err != nil {
			c.Log.Warnf("Falha no 'podman system migrate': %v", err)
		}
		c.Log.Successf("Podman e podman-compose instalados para %s (rootless).", c.User)
		return nil
	},
}

// tamanhoSubID é o tamanho padrão de faixa do useradd (SUB_UID_COUNT).
const tamanhoSubID = 65536

// garantirSubIDs dá ao usuário uma faixa em /etc/subuid ou /etc/subgid. O
// adduser já faz isso para contas locais; contas de domínio (AD/LDAP) não
// passam por ele e ficam sem faixa, e o usermod não as altera.
func garantirSubIDs(c *Ctx, arquivo string) error {
	uid, err := c.Output("id", "-u", c.User)
	if err != nil {
		return err
	}
	conteudo, err := os.ReadFile(arquivo)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	linha, falta := faixaSubID(string(conteudo), c.User, strings.TrimSpace(uid))
	if !falta {
		return nil
	}
	c.Log.Infof("Criando a faixa de IDs subordinados de %s em %s (%s).", c.User, arquivo, strings.TrimSpace(linha))
	return c.AppendFile(arquivo, linha)
}

// faixaSubID devolve a linha a acrescentar ao /etc/subuid (ou subgid) e se ela
// falta: o usuário (por nome ou UID) sem faixa recebe a primeira livre depois
// das existentes, começando em 100000 como o useradd.
func faixaSubID(conteudo, user, uid string) (string, bool) {
	inicio := 100000
	for _, l := range strings.Split(conteudo, "\n") {
		campos := strings.Split(strings.TrimSpace(l), ":")
		if len(campos) != 3 {
			continue
		}
		if campos[0] == user || campos[0] == uid {
			return "", false
		}
		ini, err1 := strconv.Atoi(campos[1])
		n, err2 := strconv.Atoi(campos[2])
		if err1 == nil && err2 == nil && ini+n > inicio {
			inicio = ini + n
		}
	}
	return fmt.Sprintf("%s:%d:%d\n", user, inicio, tamanhoSubID), true
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

// scriptNode instala o Node LTS e o pnpm pelo mise e tira dos rc a
// inicialização do fnm que versões anteriores escreviam.
const scriptNode = `set -e
mise use -g node@lts pnpm@latest

for rc in "$HOME/.zshrc" "$HOME/.bashrc"; do
    [ -f "$rc" ] || continue
    sed -i -e '/^# fnm (Node) — post-install$/d' \
        -e '/^export PATH="\$HOME\/\.local\/share\/fnm:\$PATH"$/d' \
        -e '/^eval "\$(fnm env --shell [a-z]*)"$/d' "$rc"
done
`

var node = Module{
	ID:    "node",
	Grupo: GrupoDesenvolvimento,
	Title: "Node.js e pnpm (mise)",
	Desc:  "Node LTS e pnpm pelo mise, para o usuário",
	Deps:  []string{"mise"},
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando o Node LTS e o pnpm pelo mise para %s...", c.User)
		if err := c.AsUser(scriptNode); err != nil {
			return err
		}
		c.Log.Successf("Node LTS e pnpm instalados pelo mise para %s.", c.User)
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

// scriptRuby instala o Ruby pelo mise e o Bundler e o Jekyll como gems dele.
// O ~/.default-gems faz o mise reinstalá-las em cada Ruby novo. Tira dos rc o
// GEM_HOME que versões anteriores escreviam: ele desviaria as gems do Ruby do
// mise para a pasta antiga.
const scriptRuby = `set -e
for rc in "$HOME/.zshrc" "$HOME/.bashrc"; do
    [ -f "$rc" ] || continue
    sed -i -e '/^# Gems do Ruby na home do usuário (gem\/bundle sem sudo) — post-install$/d' \
        -e '/^export GEM_HOME="\$HOME\/\.local\/share\/gem"$/d' \
        -e '/^export PATH="\$GEM_HOME\/bin:\$PATH"$/d' "$rc"
done
unset GEM_HOME

for gem in bundler jekyll; do
    grep -qxF "$gem" "$HOME/.default-gems" 2>/dev/null || echo "$gem" >> "$HOME/.default-gems"
done

mise use -g ruby@latest
mise exec -- gem install --no-document bundler jekyll
mise exec -- bundle config set --global path vendor/bundle
`

var ruby = Module{
	ID:    "ruby",
	Grupo: GrupoDesenvolvimento,
	Title: "Ruby, Bundler e Jekyll",
	Desc:  "Ruby pelo mise (compilado na máquina) e, como gems dele, Bundler e Jekyll; o Bundler instala as gems de cada projeto em vendor/bundle",
	Deps:  []string{"mise"},
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando as dependências de compilação do Ruby...")
		// O que o ruby-build (usado pelo mise) pede no Ubuntu; também serve às
		// gems nativas.
		pkgs := []string{"build-essential", "autoconf", "patch", "libssl-dev", "libyaml-dev", "libreadline-dev",
			"zlib1g-dev", "libgmp-dev", "libncurses-dev", "libffi-dev", "libgdbm-dev", "uuid-dev"}
		if err := c.install(pkgs...); err != nil {
			return err
		}

		c.Log.Infof("Instalando Ruby, Bundler e Jekyll pelo mise para %s (o Ruby é compilado; leva alguns minutos)...", c.User)
		// "path vendor/bundle" (global, em ~/.bundle/config): o bundle install
		// de cada projeto grava as gems dentro do próprio projeto. O Jekyll já
		// ignora vendor/ no build, e o .gitignore do "jekyll new" também.
		if err := c.AsUser(scriptRuby); err != nil {
			return err
		}
		c.Log.Successf("Ruby (mise), Bundler e Jekyll instalados para %s (projetos em vendor/bundle).", c.User)
		return nil
	},
}

// O mise vem do repositório oficial dele: pelo extrepo no Ubuntu 22.04 e
// 24.04, e pela PPA jdxcode/mise do 26.04 em diante, como manda a
// documentação (https://mise.jdx.dev/installing-mise.html).
var mise = Module{
	ID:    "mise",
	Grupo: GrupoDesenvolvimento,
	Title: "mise",
	Desc:  "Gerenciador de versões de ferramentas (Node, pnpm, Ruby, Rust...) pelo repositório oficial, ativado no .zshrc e no .bashrc",
	Run: func(c *Ctx) error {
		var repo []func() error
		if misePelaPPA(c.Sys.VersionID) {
			c.Log.Infof("Instalando o mise pela PPA jdxcode/mise...")
			repo = []func() error{
				func() error { return c.garantir("add-apt-repository", "software-properties-common") },
				func() error { return c.Run("add-apt-repository", "-y", "ppa:jdxcode/mise") },
			}
		} else {
			c.Log.Infof("Instalando o mise pelo extrepo...")
			repo = []func() error{
				func() error { return c.install("extrepo") },
				func() error { return c.Run("extrepo", "enable", "mise") },
			}
		}
		err := step(append(repo,
			func() error { return c.Run("apt", "update") },
			func() error { return c.install("mise") },
			func() error { return c.AsUser(scriptMise) },
		)...)
		if err != nil {
			return err
		}
		c.Log.Successf("mise instalado e ativado no shell de %s.", c.User)
		return nil
	},
}

// scriptMise ativa o mise no .zshrc e no .bashrc (uma vez só). Roda depois
// do módulo shell, porque o instalador do Oh My Zsh troca o ~/.zshrc.
const scriptMise = `for rc in "$HOME/.zshrc" "$HOME/.bashrc"; do
    [ -f "$rc" ] || continue
    sh=$(basename "$rc" rc | tr -d .)
    grep -qF 'mise activate' "$rc" || printf '\n# mise (Node, pnpm, Ruby, Rust) — post-install\neval "$(mise activate %s)"\n' "$sh" >> "$rc"
done
`

// misePelaPPA: a documentação do mise indica a PPA do Ubuntu 26.04 em diante
// e o extrepo antes dele.
func misePelaPPA(versao string) bool {
	maior, menor, _ := strings.Cut(versao, ".")
	ma, err1 := strconv.Atoi(maior)
	mi, err2 := strconv.Atoi(menor)
	if err1 != nil || err2 != nil {
		return true
	}
	return ma > 26 || (ma == 26 && mi >= 4)
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
