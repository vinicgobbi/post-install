// Package sysinfo detecta a distribuição, a versão, o gerenciador de pacotes
// e o ambiente de desktop.
//
// A família "arch" é restrita ao Arch puro (ID=arch) de propósito: derivados
// (Manjaro, EndeavourOS, CachyOS...) divergem de formas diferentes (repos,
// versões de pacote, AUR helper padrão) e "parecer" Arch não garante que o
// Chaotic-AUR e o yay funcionem sem ajuste.
package sysinfo

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type Info struct {
	ID         string
	IDLike     string
	PrettyName string
	VersionID  string

	Family string // fedora | rhel | arch | debian
	PkgMgr string // dnf | pacman | apt

	RHELVersion string // só família rhel

	// Só família debian.
	Arch            string // dpkg --print-architecture
	BaseCodename    string // jammy, bookworm, ...
	DistroVersion   string // 22.04, 12, ...
	DockerDistro    string // "ubuntu" ou "debian", para as URLs do repo Docker
	MSRepoSupported bool   // a Microsoft publica repo empacotado para essa combinação

	Desktop string // kde | cinnamon | gtk

	// Avisos gerados durante a detecção (ex.: fallback de versão do Ubuntu).
	Warnings []string
}

func (i *Info) IsKDE() bool      { return i.Desktop == "kde" }
func (i *Info) IsCinnamon() bool { return i.Desktop == "cinnamon" }

// IsGNOME vale para o GNOME e outros desktops GTK que não são o Cinnamon.
func (i *Info) IsGNOME() bool { return i.Desktop == "gtk" }

// DesktopName é o nome do ambiente para mostrar ao usuário.
func (i *Info) DesktopName() string {
	switch i.Desktop {
	case "kde":
		return "KDE Plasma"
	case "cinnamon":
		return "Cinnamon"
	default:
		return "GNOME/GTK"
	}
}

// Detect lê /etc/os-release (e, no Mint, /etc/upstream-release/lsb-release)
// e devolve as informações do sistema atual.
func Detect() (*Info, error) {
	rel, err := readKV("/etc/os-release")
	if err != nil {
		return nil, fmt.Errorf("lendo /etc/os-release: %w", err)
	}
	upstream, _ := readKV("/etc/upstream-release/lsb-release")

	info, err := fromRelease(rel, upstream)
	if err != nil {
		return nil, err
	}
	if info.Family == "debian" {
		info.Arch = dpkgArch()
	}
	info.Desktop = detectDesktop()
	return info, nil
}

func fromRelease(rel, upstream map[string]string) (*Info, error) {
	info := &Info{
		ID:         rel["ID"],
		IDLike:     rel["ID_LIKE"],
		PrettyName: rel["PRETTY_NAME"],
		VersionID:  rel["VERSION_ID"],
	}
	like := func(s string) bool { return strings.Contains(info.IDLike, s) }

	switch {
	case info.ID == "fedora":
		info.Family, info.PkgMgr = "fedora", "dnf"

	case info.ID == "almalinux" || like("rhel") || like("centos"):
		info.Family, info.PkgMgr = "rhel", "dnf"
		info.RHELVersion, _, _ = strings.Cut(info.VersionID, ".")

	case info.ID == "arch":
		info.Family, info.PkgMgr = "arch", "pacman"

	case info.ID == "ubuntu" || info.ID == "debian" || like("ubuntu") || like("debian"):
		info.Family, info.PkgMgr = "debian", "apt"

		if info.ID == "ubuntu" || like("ubuntu") {
			info.DockerDistro = "ubuntu"
			// O Mint guarda o codinome do Ubuntu base em upstream-release;
			// Ubuntu puro e outros derivados expõem direto no os-release.
			if c := upstream["DISTRIB_CODENAME"]; c != "" {
				info.BaseCodename = c
			} else if c := rel["UBUNTU_CODENAME"]; c != "" {
				info.BaseCodename = c
			} else {
				info.BaseCodename = rel["VERSION_CODENAME"]
			}

			info.MSRepoSupported = true
			switch info.BaseCodename {
			case "bionic":
				info.DistroVersion = "18.04"
			case "focal":
				info.DistroVersion = "20.04"
			case "jammy":
				info.DistroVersion = "22.04"
			case "noble":
				info.DistroVersion = "24.04"
			case "resolute":
				info.DistroVersion = "26.04"
			default:
				// Versão ainda sem "prod.repo" publicado pela Microsoft (ex.: a
				// LTS recém-lançada). Os pacotes msodbcsql/mssql-tools da última
				// LTS suportada são compatíveis na prática.
				info.DistroVersion = "26.04"
				info.Warnings = append(info.Warnings, fmt.Sprintf(
					"Ubuntu %s ainda não tem repositório oficial da Microsoft; usando o repositório do Ubuntu 26.04 (compatível) para msodbcsql/mssql-tools.",
					info.BaseCodename))
			}
		} else {
			// Debian puro e derivados diretos (ex.: LMDE).
			info.DockerDistro = "debian"
			info.BaseCodename = rel["VERSION_CODENAME"]
			if info.BaseCodename == "" {
				info.BaseCodename = info.VersionID
			}
			switch info.BaseCodename {
			case "bullseye":
				info.DistroVersion, info.MSRepoSupported = "11", true
			case "bookworm":
				info.DistroVersion, info.MSRepoSupported = "12", true
			case "trixie":
				info.DistroVersion, info.MSRepoSupported = "13", true
			default:
				info.DistroVersion = info.VersionID
			}
		}

	default:
		return nil, fmt.Errorf("distribuição não suportada: %s", info.PrettyName)
	}
	return info, nil
}

// Summary é a linha "Sistema detectado: ..." mostrada no início.
func (i *Info) Summary() string {
	switch i.Family {
	case "debian":
		return fmt.Sprintf("%s (Base: %s %s / %s)", i.PrettyName, i.DockerDistro, i.DistroVersion, i.BaseCodename)
	case "rhel":
		return fmt.Sprintf("%s (Base: RHEL %s)", i.PrettyName, i.RHELVersion)
	case "arch":
		return fmt.Sprintf("%s (rolling release, pacman + Chaotic-AUR/yay)", i.PrettyName)
	default:
		return i.PrettyName
	}
}

// O programa roda via sudo, que por padrão limpa XDG_CURRENT_DESKTOP
// (env_reset), então essa variável quase nunca chega até aqui — por isso
// também checamos se o plasmashell (ou o cinnamon) está instalado.
func detectDesktop() string {
	cur := os.Getenv("XDG_CURRENT_DESKTOP")
	switch {
	case strings.Contains(cur, "KDE"):
		return "kde"
	case strings.Contains(cur, "Cinnamon"):
		return "cinnamon"
	}
	if _, err := exec.LookPath("plasmashell"); err == nil {
		return "kde"
	}
	if _, err := exec.LookPath("cinnamon"); err == nil {
		return "cinnamon"
	}
	return "gtk"
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
