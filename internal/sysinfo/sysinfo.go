// Package sysinfo detecta a versão do Ubuntu e confere se o sistema é
// suportado.
//
// Só o Ubuntu puro (ID=ubuntu) com GNOME é suportado de propósito: derivados
// (Mint, Pop!_OS, Zorin...) e sabores com outro desktop (Kubuntu, Xubuntu...)
// divergem em repositórios, pacotes e configurações do desktop, e "parecer"
// Ubuntu não garante que os módulos funcionem sem ajuste.
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

type Info struct {
	ID         string
	PrettyName string
	VersionID  string

	Arch          string // dpkg --print-architecture
	Codename      string // jammy, noble, resolute, ...
	MSRepoVersion string // versão do Ubuntu nas URLs do repositório da Microsoft

	// Avisos gerados durante a detecção (ex.: fallback do repositório da
	// Microsoft).
	Warnings []string
}

// msRepo mapeia as versões suportadas para a versão usada nas URLs do
// repositório da Microsoft (msodbcsql/mssql-tools).
var msRepo = map[string]string{
	"jammy":    "22.04",
	"noble":    "24.04",
	"resolute": "26.04",
}

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
	// O sudo costuma limpar XDG_CURRENT_DESKTOP (env_reset), então confere
	// pelo binário do GNOME Shell.
	if _, err := exec.LookPath("gnome-shell"); err != nil {
		return nil, fmt.Errorf("%s sem o GNOME Shell não é suportado (só o Ubuntu com GNOME)", info.PrettyName)
	}
	info.Arch = dpkgArch()
	return info, nil
}

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
	if info.ID != "ubuntu" {
		return nil, fmt.Errorf("distribuição não suportada: %s (só o Ubuntu)", info.PrettyName)
	}
	if info.Codename == "" {
		info.Codename = rel["VERSION_CODENAME"]
	}

	if v, ok := msRepo[info.Codename]; ok {
		info.MSRepoVersion = v
		return info, nil
	}
	if !versaoMaiorQue(info.VersionID, 26, 4) {
		return nil, fmt.Errorf("versão não suportada: %s (só o Ubuntu 22.04, 24.04, 26.04 e mais novos)", info.PrettyName)
	}
	// Versão ainda sem "prod.repo" publicado pela Microsoft. Os pacotes
	// msodbcsql/mssql-tools da última LTS suportada são compatíveis na prática.
	info.MSRepoVersion = "26.04"
	info.Warnings = append(info.Warnings, fmt.Sprintf(
		"Ubuntu %s ainda não tem repositório oficial da Microsoft; usando o repositório do Ubuntu 26.04 (compatível) para msodbcsql/mssql-tools.",
		info.VersionID))
	return info, nil
}

// versaoMaiorQue diz se uma versão "AA.MM" é posterior a maior.menor.
func versaoMaiorQue(versao string, maior, menor int) bool {
	a, m, _ := strings.Cut(versao, ".")
	ma, err1 := strconv.Atoi(a)
	mi, err2 := strconv.Atoi(m)
	if err1 != nil || err2 != nil {
		return false
	}
	return ma > maior || (ma == maior && mi > menor)
}

// Summary é a linha "Sistema detectado: ..." mostrada no início.
func (i *Info) Summary() string {
	return fmt.Sprintf("%s (%s)", i.PrettyName, i.Codename)
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
