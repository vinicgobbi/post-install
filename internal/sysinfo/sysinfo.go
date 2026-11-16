// Package sysinfo detecta a distribuição e a versão e confere se o sistema
// é suportado.
//
// São suportados de propósito só o Ubuntu (ID=ubuntu) com GNOME e o Kubuntu
// (o mesmo ID, com KDE Plasma), o Fedora Workstation (GNOME) e o Fedora KDE
// Plasma, o Linux Mint com Cinnamon, o Zorin OS Core (com GNOME) e o CachyOS
// com KDE Plasma ou GNOME. Outros derivados (Pop!_OS, elementary...), as
// outras edições do Mint e do Zorin (MATE, Xfce, Lite, LMDE), os outros
// sabores e spins (Xubuntu, Fedora Xfce...) e as variantes atômicas do
// Fedora (Silverblue, Kinoite)
// divergem em repositórios, pacotes e configurações do
// desktop, e "parecer" suportado não garante que os módulos funcionem sem
// ajuste. As versões aceitas ficam em versoes.go; o Mint e o Zorin seguem as
// da base Ubuntu (UBUNTU_CODENAME); o CachyOS é rolling, sem versões.
package sysinfo

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// Familia é a distribuição base, que decide como cada módulo trabalha.
type Familia string

const (
	Ubuntu Familia = "ubuntu"
	Fedora Familia = "fedora"
	// Arch é a família do CachyOS (pacman, AUR pelo paru), rolling.
	Arch Familia = "arch"
)

// Desktop é o ambiente gráfico suportado na distribuição.
type Desktop string

const (
	GNOME    Desktop = "GNOME"
	Cinnamon Desktop = "Cinnamon"
	KDE      Desktop = "KDE"
)

// IDs (do /etc/os-release) dos derivados do Ubuntu suportados.
const (
	IDMint  = "linuxmint"
	IDZorin = "zorin"
)

// IDCachyOS é o ID do CachyOS no /etc/os-release (família Arch).
const IDCachyOS = "cachyos"

// fedoraHabilitado libera o Fedora Workstation em Detect (desligá-lo faz o
// programa voltar a recusar o Fedora, mantendo a detecção).
const fedoraHabilitado = true

type Info struct {
	Familia    Familia
	ID         string
	PrettyName string
	VersionID  string

	Arch     string // dpkg --print-architecture (ou a do Go, sem dpkg)
	Codename string // codinome do Ubuntu (base): jammy, noble, resolute... (vazio no Fedora)
	// UbuntuVersion é a versão do Ubuntu (no Mint e no Zorin, a da base:
	// o Mint 22.2 é "24.04"). Vazio no Fedora.
	UbuntuVersion string
	// Desktop é o ambiente gráfico: Cinnamon no Mint, KDE no Kubuntu, no
	// Fedora KDE e no CachyOS com Plasma, GNOME nas outras.
	Desktop       Desktop
	MSRepoVersion string // versão nas URLs do repositório da Microsoft (Ubuntu: 26.04...; Fedora: a do RHEL)

	// NaoTestada marca uma versão mais nova que todas as de versoes.go:
	// pode funcionar, mas é preciso confirmar antes de seguir.
	NaoTestada bool
	// Testadas são as versões de versoes.go da família, para as mensagens.
	Testadas []string

	// Avisos gerados durante a detecção (ex.: fallback do repositório da
	// Microsoft, versão não testada).
	Warnings []string
}

type versaoUbuntu struct{ Versao, Codinome, MSRepo string }

type versaoFedora struct{ Versao, MSRepo string }

// Detect lê /etc/os-release e devolve as informações do sistema atual.
func Detect() (*Info, error) {
	rel, err := readKV("/etc/os-release")
	if err != nil {
		return nil, fmt.Errorf("lendo /etc/os-release: %w", err)
	}
	info, err := fromRelease(rel)
	if err != nil {
		return nil, err
	}
	if info.Familia == Fedora {
		// As variantes atômicas declaram VARIANT_ID próprio, mas um
		// os-release mexido não deve passar: o /usr delas é só leitura.
		if _, err := os.Stat("/run/ostree-booted"); err == nil {
			return nil, fmt.Errorf("%s é uma variante atômica (ostree) do Fedora, que não é suportada (só o Fedora Workstation)", info.PrettyName)
		}
		if !fedoraHabilitado {
			return nil, fmt.Errorf("%s: o suporte ao Fedora Workstation ainda está em desenvolvimento (por enquanto, só o Ubuntu)", info.PrettyName)
		}
	}
	if err := conferirDesktop(info, func(bin string) bool {
		_, err := exec.LookPath(bin)
		return err == nil
	}); err != nil {
		return nil, err
	}
	info.Arch = dpkgArch()
	return info, nil
}

// conferirDesktop exige o ambiente gráfico suportado na distribuição: o
// Cinnamon no Mint, o GNOME ou o KDE Plasma no Ubuntu (Kubuntu) e no CachyOS
// e o GNOME no Zorin. O sudo costuma limpar
// XDG_CURRENT_DESKTOP (env_reset), então confere pelo binário.
func conferirDesktop(info *Info, tem func(bin string) bool) error {
	if info.Familia == Fedora && info.Desktop == KDE {
		// A edição KDE do Fedora (VARIANT_ID=kde, veja fromFedora).
		if !tem("plasmashell") {
			return fmt.Errorf("%s sem o KDE Plasma não é suportado", info.PrettyName)
		}
		return nil
	}
	if info.Familia == Arch {
		// O CachyOS oferece vários desktops na instalação; só o KDE Plasma
		// (o padrão) e o GNOME são suportados. Com os dois instalados, vale
		// o KDE.
		switch {
		case tem("plasmashell"):
			info.Desktop = KDE
		case tem("gnome-shell"):
			info.Desktop = GNOME
		default:
			return fmt.Errorf("%s sem o KDE Plasma nem o GNOME não é suportado (só com KDE Plasma ou GNOME)", info.PrettyName)
		}
		return nil
	}
	if info.Mint() {
		if !tem("cinnamon") {
			return fmt.Errorf("%s sem o Cinnamon não é suportado (só a edição Cinnamon do Linux Mint)", info.PrettyName)
		}
		info.Desktop = Cinnamon
		return nil
	}
	if info.ID == "ubuntu" && !tem("gnome-shell") && tem("plasmashell") {
		// O Kubuntu declara o mesmo os-release do Ubuntu: só o desktop
		// instalado o distingue. Com os dois, vale o GNOME (o do Ubuntu).
		info.Desktop = KDE
		return nil
	}
	if !tem("gnome-shell") {
		if info.ID == IDZorin {
			return fmt.Errorf("%s sem o GNOME Shell não é suportado (só o Zorin OS Core, não o Lite)", info.PrettyName)
		}
		return fmt.Errorf("%s sem o GNOME Shell nem o KDE Plasma não é suportado (só o Ubuntu com GNOME e o Kubuntu)", info.PrettyName)
	}
	info.Desktop = GNOME
	return nil
}

// Mint diz se o sistema é o Linux Mint (com Cinnamon).
func (i *Info) Mint() bool { return i.ID == IDMint }

// Derivado diz se o sistema é um derivado do Ubuntu (Mint ou Zorin).
func (i *Info) Derivado() bool { return i.ID == IDMint || i.ID == IDZorin }

func fromRelease(rel map[string]string) (*Info, error) {
	info := &Info{
		ID:         rel["ID"],
		PrettyName: rel["PRETTY_NAME"],
		VersionID:  rel["VERSION_ID"],
		Codename:   rel["UBUNTU_CODENAME"],
	}
	if info.PrettyName == "" {
		info.PrettyName = info.ID
	}
	switch info.ID {
	case "ubuntu":
		info.Familia = Ubuntu
		return fromUbuntu(info, rel)
	case IDMint, IDZorin:
		info.Familia = Ubuntu
		return fromDerivado(info)
	case "fedora":
		info.Familia = Fedora
		return fromFedora(info, rel)
	case IDCachyOS:
		// Rolling: não há versões a conferir (nem versão "não testada").
		info.Familia = Arch
		return info, nil
	}
	return nil, fmt.Errorf("distribuição não suportada: %s (só o Ubuntu, o Kubuntu, o Fedora Workstation e KDE, o Linux Mint Cinnamon, o Zorin OS Core e o CachyOS)", info.PrettyName)
}

func fromUbuntu(info *Info, rel map[string]string) (*Info, error) {
	if info.Codename == "" {
		info.Codename = rel["VERSION_CODENAME"]
	}
	for _, v := range ubuntuSuportadas {
		info.Testadas = append(info.Testadas, v.Versao)
	}
	info.UbuntuVersion = info.VersionID
	for _, v := range ubuntuSuportadas {
		if v.Codinome == info.Codename {
			info.MSRepoVersion = v.MSRepo
			return info, nil
		}
	}
	ultima := ubuntuSuportadas[len(ubuntuSuportadas)-1]
	if compararVersoes(info.VersionID, ultima.Versao) <= 0 {
		return nil, fmt.Errorf("versão não suportada: %s (só as LTS do Ubuntu em suporte: %s; e as mais novas, com confirmação)",
			info.PrettyName, juntar(info.Testadas))
	}
	// Versão ainda sem "prod.repo" publicado pela Microsoft. Os pacotes
	// msodbcsql/mssql-tools da última LTS suportada são compatíveis na prática.
	info.MSRepoVersion = ultima.MSRepo
	info.Warnings = append(info.Warnings, fmt.Sprintf(
		"Ubuntu %s ainda não tem repositório oficial da Microsoft; usando o repositório do Ubuntu %s (compatível) para msodbcsql/mssql-tools.",
		info.VersionID, ultima.MSRepo))
	naoTestada(info, "Ubuntu "+info.VersionID)
	return info, nil
}

// fromDerivado trata o Mint e o Zorin pela base Ubuntu, que eles declaram
// em UBUNTU_CODENAME (o VERSION_ID é a versão própria: Mint 22.2, Zorin 18).
// Só as bases de versoes.go são aceitas: sem a versão do Ubuntu, não há como
// saber se um codinome desconhecido é mais novo ou mais antigo. O LMDE (base
// Debian) não declara UBUNTU_CODENAME.
func fromDerivado(info *Info) (*Info, error) {
	for _, v := range ubuntuSuportadas {
		info.Testadas = append(info.Testadas, v.Versao)
	}
	if info.Codename == "" {
		return nil, fmt.Errorf("%s não é suportado: só as edições com base Ubuntu (não o LMDE, com base Debian)", info.PrettyName)
	}
	for _, v := range ubuntuSuportadas {
		if v.Codinome == info.Codename {
			info.UbuntuVersion = v.Versao
			info.MSRepoVersion = v.MSRepo
			return info, nil
		}
	}
	return nil, fmt.Errorf("versão não suportada: %s, com base no Ubuntu %q (só as bases LTS do Ubuntu em suporte: %s)",
		info.PrettyName, info.Codename, juntar(info.Testadas))
}

// fromFedora aceita as duas edições desktop: a Workstation (GNOME) e a KDE
// Plasma (VARIANT_ID=kde, cujo desktop esperado já fica em Desktop para o
// conferirDesktop exigir o Plasma).
func fromFedora(info *Info, rel map[string]string) (*Info, error) {
	switch rel["VARIANT_ID"] {
	case "workstation":
	case "kde":
		info.Desktop = KDE
	default:
		return nil, fmt.Errorf("%s não é suportado: só o Fedora Workstation e o Fedora KDE Plasma (não as variantes atômicas como Silverblue e Kinoite, os outros spins nem o Server)", info.PrettyName)
	}
	for _, v := range fedoraSuportadas {
		info.Testadas = append(info.Testadas, v.Versao)
	}
	for _, v := range fedoraSuportadas {
		if v.Versao == info.VersionID {
			info.MSRepoVersion = v.MSRepo
			return info, nil
		}
	}
	ultima := fedoraSuportadas[len(fedoraSuportadas)-1]
	if compararVersoes(info.VersionID, ultima.Versao) <= 0 {
		return nil, fmt.Errorf("versão não suportada: %s (só as versões do Fedora em manutenção: %s; e as mais novas, com confirmação)",
			info.PrettyName, juntar(info.Testadas))
	}
	info.MSRepoVersion = ultima.MSRepo
	naoTestada(info, "Fedora "+info.VersionID)
	return info, nil
}

// naoTestada marca a versão como mais nova que as de versoes.go.
func naoTestada(info *Info, nome string) {
	info.NaoTestada = true
	info.Warnings = append(info.Warnings, fmt.Sprintf(
		"%s ainda não foi testado com esta versão do post-install (testadas: %s). Continuar pode falhar ou deixar o sistema num estado incerto.",
		nome, juntar(info.Testadas)))
}

// compararVersoes compara versões como "26.04" e "45" numericamente, parte
// por parte: devolve -1, 0 ou 1. Partes que não são números valem 0.
func compararVersoes(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < max(len(pa), len(pb)); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			if x > y {
				return 1
			}
			return -1
		}
	}
	return 0
}

// juntar escreve uma lista como "a, b e c".
func juntar(itens []string) string {
	if len(itens) <= 1 {
		return strings.Join(itens, "")
	}
	return strings.Join(itens[:len(itens)-1], ", ") + " e " + itens[len(itens)-1]
}

// Summary é a linha "Sistema detectado: ..." mostrada no início.
func (i *Info) Summary() string {
	if i.Familia == Arch && i.Desktop != "" {
		return fmt.Sprintf("%s (%s)", i.PrettyName, nomeDesktop(i.Desktop))
	}
	if i.Derivado() && i.UbuntuVersion != "" {
		return fmt.Sprintf("%s (base Ubuntu %s, %s)", i.PrettyName, i.UbuntuVersion, i.Codename)
	}
	if i.Codename == "" {
		return i.PrettyName
	}
	if i.ID == "ubuntu" && i.Desktop == KDE {
		// O PRETTY_NAME do Kubuntu é o do Ubuntu.
		return fmt.Sprintf("Kubuntu (%s, %s)", i.PrettyName, i.Codename)
	}
	return fmt.Sprintf("%s (%s)", i.PrettyName, i.Codename)
}

// nomeDesktop é o nome do desktop para as mensagens.
func nomeDesktop(d Desktop) string {
	if d == KDE {
		return "KDE Plasma"
	}
	return string(d)
}

func dpkgArch() string {
	if out, err := exec.Command("dpkg", "--print-architecture").Output(); err == nil {
		return strings.TrimSpace(string(out))
	}
	switch runtime.GOARCH {
	case "arm":
		return "armhf"
	default:
		return runtime.GOARCH
	}
}

// readKV lê arquivos no formato CHAVE=valor (com ou sem aspas).
func readKV(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseKV(bufio.NewScanner(f)), nil
}

func parseKV(sc *bufio.Scanner) map[string]string {
	m := map[string]string{}
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		m[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return m
}
