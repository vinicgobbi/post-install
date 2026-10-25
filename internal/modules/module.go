// Package modules contém as etapas da pós-instalação. Cada módulo pertence a
// um grupo e é autocontido (configura o próprio repositório, se precisar);
// a ordem de execução é a ordem da lista em All.
package modules

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/vinicgobbi/post-install/internal/runner"
	"github.com/vinicgobbi/post-install/internal/sysinfo"
)

// Grupos, na ordem em que aparecem na interface e rodam.
const (
	GrupoSistema         = "Sistema"
	GrupoTerminal        = "Terminal"
	GrupoAplicativos     = "Aplicativos"
	GrupoDesenvolvimento = "Desenvolvimento"
	GrupoRede            = "Rede e virtualização"
	GrupoGNOME           = "GNOME e aparência"
	GrupoFinalizacao     = "Finalização"
)

type Module struct {
	ID    string
	Grupo string
	Title string
	Desc  string
	// Deps são IDs de módulos que precisam rodar antes deste.
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
	// GNOMEDir guarda extensoes.txt, extensoes.dconf e atalhos.dconf.
	GNOMEDir string
	// ForcarExtensoes são as extensões sem versão compatível que o usuário
	// pediu para instalar mesmo assim.
	ForcarExtensoes []string
}

// Escolhas é o que o usuário decide antes da execução (na TUI ou no modo
// texto).
type Escolhas struct {
	User            string
	Wallpaper       string
	ForcarExtensoes []string
}

// All devolve todos os módulos, na ordem de execução (agrupados por Grupo).
func All() []Module {
	return []Module{
		// Sistema
		mirrors,
		atualizacao,
		// Terminal. O shell vem antes dos módulos que escrevem no ~/.zshrc
		// (node, ruby): o instalador do Oh My Zsh troca o arquivo.
		ferramentas,
		shell,
		// Aplicativos
		firefoxThunderbird,
		chrome,
		bitwarden,
		removerLibreoffice,
		flatpaks,
		flatpaksJogos,
		solaar,
		// Desenvolvimento
		gitGCM,
		docker,
		vscode,
		golang,
		node,
		phpSQLServer,
		rust,
		ruby,
		claudeCode,
		deployDev,
		// Rede e virtualização
		tailscale,
		firewall,
		ovpn,
		virtManager,
		// GNOME e aparência
		temasYaru,
		papelParede,
		extensoesGnome,
		atalhosGnome,
		nautilus,
		// Depois de tudo instalado: organiza o que existir na máquina.
		appsGnome,
		// Finalização
		limpeza,
	}
}

// ---- Helpers de pacotes ----

// install instala pacotes com o apt, de forma não interativa.
func (c *Ctx) install(pkgs ...string) error {
	return c.installEnv(nil, pkgs...)
}

func (c *Ctx) installEnv(env []string, pkgs ...string) error {
	return c.RunEnv(env, "apt", append([]string{"install", "-y"}, pkgs...)...)
}

// garantir instala pkg se o comando bin ainda não existir (ex.: o curl, que
// não vem no Ubuntu Desktop e os instaladores oficiais usam).
func (c *Ctx) garantir(bin, pkg string) error {
	if runner.Exists(bin) {
		return nil
	}
	return c.install(pkg)
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
	fprTailscale    = "2596A99EAAB33821893C0A79458CA832957F5868"
)

// instalarChaveAPT baixa uma chave para /etc/apt/keyrings (ou outro destino)
// só depois de conferir o fingerprint.
func instalarChaveAPT(c *Ctx, url, destino string, fprs ...string) error {
	tmp := c.Temp(filepath.Base(destino))
	return step(
		func() error { return c.Download(url, tmp) },
		func() error { return c.VerifyKey(tmp, fprs...) },
		func() error { return c.Run("install", "-D", "-m", "0644", tmp, destino) },
	)
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

var lookPath = exec.LookPath
