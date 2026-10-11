// Package modules contém as etapas da pós-instalação. Cada arquivo define um
// módulo; a ordem de execução é a ordem da lista em All.
package modules

import (
	"fmt"
	"strings"

	"github.com/vinicgobbi/post-install/internal/runner"
	"github.com/vinicgobbi/post-install/internal/sysinfo"
)

type Module struct {
	ID    string
	Title string
	Desc  string
	// Deps são IDs de módulos que precisam rodar antes deste (ex.: as
	// extensões PHP precisam dos headers do driver ODBC de "pacotes_base").
	Deps []string
	Run  func(c *Ctx) error
}

// Ctx é o que cada módulo recebe para trabalhar.
type Ctx struct {
	*runner.Runner
	Sys     *sysinfo.Info
	OVPNDir string
	// Wallpaper é o caminho da imagem escolhida (vazio = não alterar).
	Wallpaper string
}

// All devolve todos os módulos, na ordem de execução.
func All() []Module {
	return []Module{
		mirrors,
		atualizacao,
		repositorios,
		pacotesBase,
		solaar,
		removerLibreoffice,
		flatpaks,
		flatpaksJogos,
		tailscale,
		phpExtensoes,
		chromeGCM,
		bitwarden,
		temasIcones,
		ambienteUsuario,
		papelParede,
		rustTools,
		claudeCode,
		vscodeNautilus,
		ovpn,
		virtManager,
		limpeza,
	}
}

// ---- Helpers por gerenciador de pacotes ----

// install instala pacotes com o gerenciador nativo, de forma não interativa.
func (c *Ctx) install(pkgs ...string) error {
	return c.installEnv(nil, pkgs...)
}

func (c *Ctx) installEnv(env []string, pkgs ...string) error {
	switch c.Sys.PkgMgr {
	case "dnf":
		return c.RunEnv(env, "dnf", append([]string{"install", "-y"}, pkgs...)...)
	case "pacman":
		return c.RunEnv(env, "pacman", append([]string{"-S", "--needed", "--noconfirm"}, pkgs...)...)
	default:
		return c.RunEnv(env, "apt", append([]string{"install", "-y"}, pkgs...)...)
	}
}

// installAUR instala pacotes via yay como o usuário alvo — o makepkg recusa
// rodar como root. Resolve tanto binários do Chaotic-AUR quanto pacotes que
// só existem na AUR (compila). Depende do módulo "repositorios" ter liberado
// sudo sem senha para o pacman nesse usuário.
func (c *Ctx) installAUR(pkgs ...string) error {
	return c.AsUser("yay -S --needed --noconfirm " + strings.Join(pkgs, " "))
}

func (c *Ctx) ensureFlathub() error {
	return c.Run("flatpak", "remote-add", "--if-not-exists", "flathub", "https://flathub.org/repo/flathub.flatpakrepo")
}

func (c *Ctx) flatpakInstall(apps ...string) error {
	if len(apps) == 0 {
		return nil
	}
	return c.Run("flatpak", append([]string{"install", "-y", "flathub"}, apps...)...)
}

// addUserToGroup adiciona o usuário alvo a um grupo, se o grupo existir.
func (c *Ctx) addUserToGroup(group string) error {
	if !c.Probe("getent", "group", group) {
		return nil
	}
	return c.Run("usermod", "-aG", group, c.User)
}

// step encadeia passos, parando no primeiro erro (como o "set -e" do bash).
func step(fns ...func() error) error {
	for _, fn := range fns {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}

func errorf(format string, a ...any) error { return fmt.Errorf(format, a...) }
