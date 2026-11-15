// Package modules contém as etapas da pós-instalação. Cada módulo pertence a
// um grupo e é autocontido (configura o próprio repositório, se precisar);
// a ordem de execução é a ordem da lista em All. O Run de cada módulo é o
// caminho do Ubuntu (e do Zorin OS); a variante do Fedora fica no campo
// Fedora, as do Linux Mint (Cinnamon) e do Zorin OS nos campos Mint e Zorin,
// a do CachyOS no campo Arch, a do KDE Plasma (CachyOS e Fedora KDE) no
// campo KDE, e Disponiveis devolve a
// lista já adaptada à distribuição detectada.
package modules

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

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
	// Run é o caminho do Ubuntu.
	Run func(c *Ctx) error
	// Fedora é a variante do Fedora Workstation (nil = indisponível nele).
	Fedora *Variante
	// Mint é a variante do Linux Mint com Cinnamon, que é da família Ubuntu:
	// nil = igual ao Ubuntu; Indisponivel = some no Mint.
	Mint *Variante
	// Zorin é a variante do Zorin OS (GNOME, família Ubuntu), como a do Mint.
	Zorin *Variante
	// Arch é a variante da família Arch (CachyOS) — nil = indisponível nela,
	// como a do Fedora.
	Arch *Variante
	// KDE é a variante do KDE Plasma (CachyOS e Fedora KDE), por cima da da
	// família (nil = igual à da família; Indisponivel = some no KDE). Ela só
	// usa o que é igual nas duas distribuições (pacotes de mesmo nome).
	KDE *Variante
}

// Variante é como o módulo trabalha numa distribuição que não o Ubuntu.
// Title e Desc vazios e Deps nil herdam os do módulo (Deps vazio, mas não
// nil, tira as dependências). Run nil reaproveita o do Ubuntu, para módulos
// que já funcionam igual (só usam c.install, que escolhe o apt ou o dnf, o
// mise, o curl...).
type Variante struct {
	Title string
	Desc  string
	Deps  []string
	Run   func(c *Ctx) error
	// Indisponivel tira o módulo da distribuição (ex.: as extensões do GNOME
	// no Mint, que usa o Cinnamon).
	Indisponivel bool
}

// Para devolve o módulo como ele roda na família dada, e se está disponível
// nela.
func (m Module) Para(f sysinfo.Familia) (Module, bool) {
	switch f {
	case sysinfo.Ubuntu, "":
		return m, true
	case sysinfo.Fedora:
		return m.aplicar(m.Fedora)
	case sysinfo.Arch:
		return m.aplicar(m.Arch)
	}
	return Module{}, false
}

// ParaSistema é o Para da família do sistema e, no Linux Mint e no Zorin
// OS, com a variante deles por cima (sem ela, vale o módulo do Ubuntu); com o
// KDE Plasma (CachyOS e Fedora KDE), a variante do KDE por cima da da
// família.
func (m Module) ParaSistema(sys *sysinfo.Info) (Module, bool) {
	if sys.Desktop == sysinfo.KDE && m.KDE != nil {
		return m.aplicar(m.KDE)
	}
	if sys.Mint() && m.Mint != nil {
		return m.aplicar(m.Mint)
	}
	if sys.ID == sysinfo.IDZorin && m.Zorin != nil {
		return m.aplicar(m.Zorin)
	}
	return m.Para(sys.Familia)
}

// aplicar devolve o módulo com a variante v (nil = indisponível).
func (m Module) aplicar(v *Variante) (Module, bool) {
	if v == nil || v.Indisponivel {
		return Module{}, false
	}
	out := m
	out.Fedora, out.Mint, out.Zorin, out.Arch, out.KDE = nil, nil, nil, nil, nil
	if v.Run != nil {
		out.Run = v.Run
	}
	if v.Title != "" {
		out.Title = v.Title
	}
	if v.Desc != "" {
		out.Desc = v.Desc
	}
	if v.Deps != nil {
		out.Deps = v.Deps
	}
	return out, true
}

// Disponiveis devolve os módulos disponíveis na distribuição, já adaptados
// a ela, na ordem de All. No Ubuntu é a própria All.
func Disponiveis(sys *sysinfo.Info) []Module {
	var out []Module
	for _, m := range All() {
		if mm, ok := m.ParaSistema(sys); ok {
			out = append(out, mm)
		}
	}
	return out
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
		// (mise, node, ruby): o instalador do Oh My Zsh troca o arquivo.
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
		podman,
		vscode,
		golang,
		mise,
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

// fedora diz se o sistema é da família Fedora (sem Sys, vale o Ubuntu).
func (c *Ctx) fedora() bool { return c.Sys != nil && c.Sys.Familia == sysinfo.Fedora }

// arch diz se o sistema é da família Arch (CachyOS).
func (c *Ctx) arch() bool { return c.Sys != nil && c.Sys.Familia == sysinfo.Arch }

// kde diz se o desktop é o KDE Plasma (CachyOS ou Fedora KDE).
func (c *Ctx) kde() bool { return c.Sys != nil && c.Sys.Desktop == sysinfo.KDE }

// mint diz se o sistema é o Linux Mint (com Cinnamon).
func (c *Ctx) mint() bool { return c.Sys != nil && c.Sys.Mint() }

// install instala pacotes de forma não interativa, com o apt no Ubuntu, o
// dnf no Fedora e o pacman no Arch (só dos repositórios oficiais; o AUR é
// com instalarAUR). Os nomes são os da distribuição: cada variante passa os
// seus. Também aceita o caminho de um .deb/.rpm baixado.
func (c *Ctx) install(pkgs ...string) error {
	return c.installEnv(nil, pkgs...)
}

func (c *Ctx) installEnv(env []string, pkgs ...string) error {
	if c.arch() {
		return c.RunEnv(env, "pacman", append([]string{"-S", "--needed", "--noconfirm"}, pkgs...)...)
	}
	if c.fedora() {
		return c.RunEnv(env, "dnf", append([]string{"install", "-y"}, pkgs...)...)
	}
	return c.RunEnv(env, "apt", append([]string{"install", "-y"}, pkgs...)...)
}

// pacoteInstalado consulta o gerenciador de pacotes (dpkg, rpm ou pacman).
func (c *Ctx) pacoteInstalado(pkg string) bool {
	if c.arch() {
		return c.Probe("pacman", "-Q", pkg)
	}
	if c.fedora() {
		return c.Probe("rpm", "-q", pkg)
	}
	return c.Probe("dpkg", "-s", pkg)
}

// sudoersAUR libera, só durante instalarAUR, o pacman sem senha para o
// usuário alvo: o paru compila como usuário comum e chama o "sudo pacman"
// no fim, o que pediria a senha sem um terminal para digitá-la.
const sudoersAUR = "/etc/sudoers.d/90-post-install-aur"

// instalarAUR instala pacotes do AUR com o paru (que o CachyOS traz), como o
// usuário alvo: o makepkg não roda como root. --skipreview pula a revisão
// interativa do PKGBUILD. O sudoers temporário é validado pelo visudo antes
// de entrar no lugar e removido no fim, mesmo se a instalação falhar.
func (c *Ctx) instalarAUR(pkgs ...string) (err error) {
	if err := c.garantir("paru", "paru"); err != nil {
		return err
	}
	tmp := c.Temp("sudoers-aur")
	regra := c.User + " ALL=(root) NOPASSWD: /usr/bin/pacman\n"
	defer func() {
		if rmErr := c.RemoveAll(sudoersAUR); rmErr != nil && err == nil {
			err = errorf("não foi possível remover %s: %w", sudoersAUR, rmErr)
		}
	}()
	args := make([]string, len(pkgs))
	for i, p := range pkgs {
		args[i] = shQuote(p)
	}
	return step(
		func() error { return c.WriteFile(tmp, regra, 0o440) },
		func() error { return c.Run("visudo", "-cf", tmp) },
		func() error { return c.Run("install", "-m", "0440", "-o", "root", "-g", "root", tmp, sudoersAUR) },
		func() error {
			return c.AsUser("paru -S --needed --noconfirm --skipreview " + strings.Join(args, " "))
		},
	)
}

// garantir instala pkg se o comando bin ainda não existir (ex.: o curl, que
// não vem no Ubuntu Desktop e os instaladores oficiais usam).
func (c *Ctx) garantir(bin, pkg string) error {
	if c.Exists(bin) {
		return nil
	}
	return c.install(pkg)
}

// Fingerprints das chaves primárias que assinam cada repositório. As chaves
// são baixadas dos sites oficiais e conferidas contra estes valores antes de
// serem instaladas, para um download adulterado não virar repositório
// confiável do sistema.
const (
	fprMicrosoft = "BC528686B50D79E339D3721CEB3E94ADBE1229CF"
	// O keyring do GitHub CLI traz a chave atual e a anterior (expirada).
	fprGitHubCLI    = "7F38BBB59D064DBCB3D84D725612B36462313325"
	fprGitHubCLIOld = "2C6106201985B60E6C7AC87323F3D4EA75716059"
	fprTailscale    = "2596A99EAAB33821893C0A79458CA832957F5868"
	// Google Linux Packages Signing Authority (Chrome).
	fprGoogle = "EB4C1BFD4F042F6DDDCCEC917721F63BD38B4796"
	// Chave de 2025 da Microsoft, que assina os repositórios do RHEL 10
	// (packages.microsoft.com/keys/microsoft-2025.asc; o fingerprint também
	// está em learn.microsoft.com/linux/packages).
	fprMicrosoft2025 = "AA86F75E427A19DD33346403EE4D7792F748182B"
	// mise releases <release@mise.jdx.dev> (mise.jdx.dev/gpg-key.pub, o
	// mesmo fingerprint publicado no keys.openpgp.org).
	fprMise = "24853EC9F655CE80B48E6C3A8B81C9D17413A06D"
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

// chavesRPM é onde as chaves dos repositórios RPM ficam, como no Fedora.
const chavesRPM = "/etc/pki/rpm-gpg"

// instalarChaveRPM baixa a chave de um repositório RPM para chavesRPM, só
// depois de conferir o fingerprint. O dnf a importa sozinho na primeira
// instalação pelo repositório (gpgkey do .repo, veja repoRPM). Não usa
// "rpm --import": ele pode ler só a primeira chave de um arquivo com várias
// (ex.: o do GitHub CLI, em que a primeira é a antiga, expirada).
func instalarChaveRPM(c *Ctx, url, nome string, fprs ...string) (string, error) {
	destino := filepath.Join(chavesRPM, nome)
	tmp := c.Temp(nome)
	return destino, step(
		func() error { return c.Download(url, tmp) },
		func() error { return c.VerifyKey(tmp, fprs...) },
		func() error { return c.Run("install", "-D", "-m", "0644", tmp, destino) },
	)
}

// repoRPM grava /etc/yum.repos.d/<nome>.repo, com a assinatura dos pacotes
// conferida pela chave local (gpgkey=file://...). O nome é o mesmo arquivo
// das instruções oficiais de cada repositório, para quem já o configurou à
// mão não ficar com o repositório duplicado. extras são linhas a mais (ex.:
// "repo_gpgcheck=1").
func repoRPM(c *Ctx, nome, titulo, baseurl, chave string, extras ...string) error {
	return c.WriteFile("/etc/yum.repos.d/"+nome+".repo", conteudoRepoRPM(nome, titulo, baseurl, chave, extras...), 0o644)
}

func conteudoRepoRPM(nome, titulo, baseurl, chave string, extras ...string) string {
	s := "[" + nome + "]\n" +
		"name=" + titulo + "\n" +
		"baseurl=" + baseurl + "\n" +
		"enabled=1\n" +
		"gpgcheck=1\n" +
		"gpgkey=file://" + chave + "\n"
	for _, e := range extras {
		s += e + "\n"
	}
	return s
}

func (c *Ctx) ensureFlathub() error {
	return c.Run("flatpak", "remote-add", "--if-not-exists", "flathub", "https://flathub.org/repo/flathub.flatpakrepo")
}

// ensureFlathubFedora: o Fedora pode trazer o Flathub já cadastrado, mas
// desativado ou filtrado (só alguns apps); o remote-modify o deixa completo.
func (c *Ctx) ensureFlathubFedora() error {
	return step(c.ensureFlathub, func() error {
		return c.Run("flatpak", "remote-modify", "--enable", "--no-filter", "flathub")
	})
}

// garantirFlathub cadastra o Flathub (no Fedora, também o ativa e tira o
// filtro).
func (c *Ctx) garantirFlathub() error {
	if c.fedora() {
		return c.ensureFlathubFedora()
	}
	return c.ensureFlathub()
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

// arquivos é o acesso de leitura à máquina usado pelos leitores de
// configuração e de .desktop. O *runner.Runner implementa (com Host nil, lê
// a máquina real; nos testes, um Host falso).
type arquivos interface {
	ReadFile(path string) ([]byte, error)
	Stat(path string) (fs.FileInfo, error)
	WalkDir(root string, fn fs.WalkDirFunc) error
	LookPath(name string) (string, error)
}

// maquinaReal lê a máquina real, para quem roda fora de um módulo (ex.: a
// verificação de extensões antes da interface).
var maquinaReal arquivos = &runner.Runner{}
