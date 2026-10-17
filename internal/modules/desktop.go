package modules

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var temasIcones = Module{
	ID:    "temas_icones",
	Title: "Temas e ícones",
	Desc:  "Instala o tema adw-gtk3 e os ícones Yaru (pulado automaticamente no Plasma)",
	Run: func(c *Ctx) error {
		// adw-gtk3 e Yaru existem só para dar cara de GNOME a apps GTK; no
		// Plasma o padrão já é Breeze.
		if c.Sys.IsKDE() {
			c.Log.Infof("Ambiente Plasma detectado: pulando temas/ícones GTK (adw-gtk3/Yaru), sem uso no KDE.")
			return nil
		}

		if c.Sys.PkgMgr == "pacman" {
			c.Log.Infof("Instalando adw-gtk-theme (oficial) e ícones Yaru (AUR/Chaotic-AUR)...")
			if err := step(
				func() error { return c.install("adw-gtk-theme") },
				func() error { return c.installAUR("yaru-icon-theme") },
			); err != nil {
				return err
			}
			c.Log.Successf("Temas e ícones implantados no sistema.")
			return nil
		}

		c.Log.Infof("Configurando temas e ícones globais em /usr/share...")
		if err := step(
			func() error { return c.MkdirAll("/usr/share/themes", 0o755) },
			func() error { return c.MkdirAll("/usr/share/icons", 0o755) },
		); err != nil {
			return err
		}

		if c.Sys.ID != "ubuntu" {
			if err := instalarAdwGtk3(c); err != nil {
				return err
			}
		}
		if err := instalarYaru(c); err != nil {
			return err
		}
		c.Log.Successf("Temas e ícones implantados no sistema.")
		return nil
	},
}

func instalarAdwGtk3(c *Ctx) error {
	c.Log.Infof("Verificando adw-gtk3 nos repositórios...")
	pkg := ""
	if c.Sys.PkgMgr == "dnf" {
		if c.Probe("dnf", "list", "available", "adw-gtk3-theme") {
			pkg = "adw-gtk3-theme"
		}
	} else if c.Probe("apt-cache", "show", "adw-gtk3") {
		pkg = "adw-gtk3"
	}

	if pkg != "" {
		c.Log.Infof("Instalando adw-gtk3 via gerenciador de pacotes nativo...")
		return c.install(pkg)
	}

	c.Log.Infof("adw-gtk3 ausente dos repositórios. Baixando tarball oficial do GitHub...")
	url, err := latestReleaseAsset(c, "lassekongo83/adw-gtk3", func(n string) bool { return strings.HasSuffix(n, ".tar.xz") })
	if err != nil {
		c.Log.Warnf("Não foi possível localizar o tarball do adw-gtk3: %v", err)
		return nil
	}
	tarball := c.Temp("adw-gtk3.tar.xz")
	return step(
		func() error { return c.Download(url, tarball) },
		func() error { return c.Run("tar", "-xJf", tarball, "-C", "/usr/share/themes/") },
	)
}

func instalarYaru(c *Ctx) error {
	c.Log.Infof("Verificando ícones Yaru nos repositórios...")
	pkg := ""
	if c.Sys.PkgMgr == "dnf" {
		if c.Probe("dnf", "list", "available", "yaru-icon-theme") {
			pkg = "yaru-icon-theme"
		}
	} else if c.Probe("apt-cache", "show", "yaru-theme-icon") {
		pkg = "yaru-theme-icon"
	}

	if pkg != "" {
		c.Log.Infof("Instalando ícones Yaru via gerenciador de pacotes nativo...")
		return c.install(pkg)
	}

	c.Log.Infof("Ícones Yaru ausentes dos repositórios. Baixando do repositório oficial do Ubuntu...")
	tarball, dir := c.Temp("yaru.tar.gz"), c.Temp("yaru")
	err := step(
		func() error {
			return c.Download("https://github.com/ubuntu/yaru/archive/refs/heads/master.tar.gz", tarball)
		},
		func() error { return c.MkdirAll(dir, 0o755) },
		func() error { return c.Run("tar", "-xzf", tarball, "-C", dir) },
	)
	if err != nil {
		return err
	}
	if err := c.Run("cp", "-r", dir+"/yaru-master/icons/.", "/usr/share/icons/"); err != nil {
		c.Log.Warnf("Falha ao copiar os ícones Yaru: %v", err)
	}
	return nil
}

var ambienteUsuario = Module{
	ID:    "ambiente_usuario",
	Title: "Configurar ambiente do usuário",
	Desc:  "Zsh, Oh My Zsh, fnm/Node, dotfiles e ajustes visuais (GNOME ou Plasma, conforme detectado)",
	Deps:  []string{"pacotes_base"},
	Run: func(c *Ctx) error {
		c.Log.Infof("Aplicando configurações locais para %s...", c.User)
		if err := c.Run("usermod", "-aG", "docker", c.User); err != nil {
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

		if err := c.AsUser(`git-credential-manager configure && git config --global credential.credentialStore secretservice`); err != nil {
			c.Log.Warnf("Falha ao configurar o Git Credential Manager: %v", err)
		}

		// O download do fnm e o clone+bootstrap dos dotfiles (Oh My Zsh,
		// plugins, tema, fontes, config do Solaar) são independentes: rodam
		// em paralelo.
		errs := make(chan error, 2)
		go func() {
			errs <- c.AsUser(`curl -fsSL https://fnm.vercel.app/install | bash -s -- --skip-shell`)
		}()
		go func() {
			errs <- c.AsUser(`rm -rf /tmp/dotfiles && git clone https://github.com/vinicgobbi/Dotfiles.git /tmp/dotfiles && bash /tmp/dotfiles/bootstrap.sh`)
		}()
		for range 2 {
			if err := <-errs; err != nil {
				c.Log.Warnf("%v", err)
			}
		}

		if err := c.AsUser(scriptAmbienteUsuario(c)); err != nil {
			c.Log.Warnf("Parte dos ajustes do ambiente do usuário falhou: %v", err)
		}
		c.Log.Successf("Ambiente de usuário configurado.")
		return nil
	},
}

// scriptAmbienteUsuario roda depois do bootstrap dos dotfiles (que cria o
// ~/.zshrc via Oh My Zsh): instala o Node LTS via fnm, cria o diretório de
// projetos e aplica os ajustes visuais.
func scriptAmbienteUsuario(c *Ctx) string {
	visual := `gsettings set org.gnome.desktop.interface icon-theme "Yaru-dark" 2>/dev/null || true`
	if c.Sys.ID != "ubuntu" {
		visual = `gsettings set org.gnome.desktop.interface gtk-theme 'adw-gtk3-dark' && gsettings set org.gnome.desktop.interface color-scheme 'prefer-dark' 2>/dev/null || true
` + visual
	}
	if c.Sys.IsKDE() {
		visual = `command -v plasma-apply-colorscheme >/dev/null 2>&1 && plasma-apply-colorscheme BreezeDark >/dev/null 2>&1 || true`
	}

	return fmt.Sprintf(`
# Node LTS via fnm
export PATH="$HOME/.local/share/fnm:$PATH"
eval "$(fnm env --shell zsh)"
fnm install --lts
fnm default $(fnm current)

# Inicialização do fnm no .zshrc (depois do Oh My Zsh, que cria o ~/.zshrc)
grep -qF 'fnm env --shell zsh' ~/.zshrc 2>/dev/null || {
    echo 'export PATH="$HOME/.local/share/fnm:$PATH"' >> ~/.zshrc
    echo 'eval "$(fnm env --shell zsh)"' >> ~/.zshrc
}

# Diretório de projetos (XDG e bookmarks)
if [[ "$LANG" == pt_* ]]; then DIR_NAME="Projetos"; else DIR_NAME="Projects"; fi
PROJECTS_DIR="$HOME/$DIR_NAME"
mkdir -p "$PROJECTS_DIR"
xdg-user-dirs-update --set PROJECTS "$PROJECTS_DIR"

BOOKMARKS_FILE="$HOME/.config/gtk-3.0/bookmarks"
mkdir -p "$(dirname "$BOOKMARKS_FILE")"
touch "$BOOKMARKS_FILE"
grep -q "file://$PROJECTS_DIR" "$BOOKMARKS_FILE" || echo "file://$PROJECTS_DIR" >> "$BOOKMARKS_FILE"

# Autostart do Solaar (pacote nativo, ou Flatpak como alternativa), iniciando
# só no ícone da bandeja ("-w hide"), sem abrir a janela
mkdir -p "$HOME/.config/autostart"
for desktop in /usr/share/applications/solaar.desktop \
    /var/lib/flatpak/exports/share/applications/io.github.pwr_solaar.solaar.desktop; do
    if [[ -f "$desktop" ]]; then
        destino="$HOME/.config/autostart/$(basename "$desktop")"
        cp "$desktop" "$destino"
        sed -i -E '/^Exec=/{/ -w hide$/! s/$/ -w hide/}' "$destino"
        break
    fi
done

# Ajustes visuais
%s
`, visual)
}

var rustTools = Module{
	ID:    "rust_tools",
	Title: "Instalar Rust, eza e topgrade",
	Desc:  "rustup e compilação de eza e topgrade via cargo",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando rustup e compilando eza/topgrade via cargo...")
		var buildPkgs []string
		switch c.Sys.PkgMgr {
		case "dnf":
			buildPkgs = []string{"gcc", "make"}
		case "pacman":
			buildPkgs = []string{"base-devel"}
		default:
			buildPkgs = []string{"build-essential"}
		}
		err := step(
			func() error { return c.install(buildPkgs...) },
			func() error {
				return c.AsUser(`set -e
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain stable
source "$HOME/.cargo/env"
cargo install --locked eza topgrade
`)
			},
		)
		if err != nil {
			return err
		}
		c.Log.Successf("rustup, eza e topgrade instalados para %s.", c.User)
		return nil
	},
}

var claudeCode = Module{
	ID:    "claude_code",
	Title: "Instalar Claude Code",
	Desc:  "CLI oficial da Anthropic para desenvolvimento assistido por IA no terminal",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando o Claude Code...")
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
	Title: "Instalar deploy-dev",
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
		if err := c.Download(bin, dest); err != nil {
			return err
		}
		if !c.DryRun {
			if err := conferirSHA256(c, sum, dest); err != nil {
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
	f, err := os.Open(arquivo)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, campos[0]) {
		return errorf("SHA-256 de %s (%s) não confere com o da release (%s)", filepath.Base(arquivo), got, campos[0])
	}
	return nil
}

const dolphinServiceMenu = `[Desktop Entry]
Type=Service
X-KDE-ServiceTypes=KonqPopupMenu/Plugin
MimeType=all/allfiles;inode/directory;
Actions=openWithCode;

[Desktop Action openWithCode]
Name=Abrir com o VSCode
Icon=code
Exec=code %U
`

var vscodeNautilus = Module{
	ID:    "vscode_nautilus",
	Title: "Extensão VSCode no gerenciador de arquivos",
	Desc:  "Adiciona 'Abrir com o VSCode' ao menu de contexto (Nautilus no GNOME, Dolphin no Plasma) e, no Nautilus, 'Copiar caminho'",
	Run: func(c *Ctx) error {
		if c.Sys.IsKDE() {
			c.Log.Infof("Ambiente Plasma detectado: adicionando 'Abrir com o VSCode' ao menu de contexto do Dolphin...")
			if err := step(
				func() error { return c.MkdirAll("/usr/share/kio/servicemenus", 0o755) },
				func() error {
					return c.WriteFile("/usr/share/kio/servicemenus/vscode-open-with.desktop", dolphinServiceMenu, 0o644)
				},
			); err != nil {
				return err
			}
			for _, kb := range []string{"kbuildsycoca6", "kbuildsycoca5"} {
				if _, err := lookPath(kb); err == nil {
					c.Try(kb, "--noincremental")
				}
			}
			c.Log.Successf("Item 'Abrir com o VSCode' adicionado ao Dolphin.")
			return nil
		}

		c.Log.Infof("Instalando extensões do VSCode e de copiar caminho para o Nautilus...")
		var deps []string
		switch c.Sys.PkgMgr {
		case "dnf":
			deps = []string{"nautilus-python", "python3-gobject", "wl-clipboard", "xclip"}
		case "pacman":
			deps = []string{"nautilus-python", "python-gobject", "wl-clipboard", "xclip"}
		default:
			deps = []string{"python3-nautilus", "python3-gi", "wl-clipboard", "xclip"}
		}
		err := step(
			func() error { return c.install(deps...) },
			func() error {
				return c.AsUser(`set -e
rm -rf /tmp/vscode_nautilus
git clone https://github.com/vinicgobbi/vscode_nautilus.git /tmp/vscode_nautilus
bash /tmp/vscode_nautilus/install.sh

rm -rf /tmp/nautilus-copypath
git clone https://github.com/vinicgobbi/nautilus-copypath.git /tmp/nautilus-copypath
bash /tmp/nautilus-copypath/install.sh
`)
			},
		)
		if err != nil {
			return err
		}
		c.Log.Successf("Extensões do VSCode e de copiar caminho instaladas no Nautilus.")
		return nil
	},
}
