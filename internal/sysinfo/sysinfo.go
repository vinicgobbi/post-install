// Package sysinfo detecta a distribuição e a versão e confere se o sistema
// é suportado.
//
// Só o Ubuntu puro (ID=ubuntu) e o Fedora Workstation, ambos com GNOME, são
// suportados de propósito: derivados (Mint, Pop!_OS, Zorin...), sabores e
// spins com outro desktop (Kubuntu, Fedora KDE...) e as variantes atômicas
// do Fedora (Silverblue, Kinoite) divergem em repositórios, pacotes e
// configurações do desktop, e "parecer" suportado não garante que os
// módulos funcionem sem ajuste. As versões aceitas ficam em versoes.go.
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
)

// fedoraHabilitado libera o Fedora Workstation em Detect. Fica desligado
// até os módulos terem a variante do Fedora: até lá a detecção existe (e é
// testada), mas o programa recusa o sistema.
const fedoraHabilitado = false

type Info struct {
	Familia    Familia
	ID         string
	PrettyName string
	VersionID  string

	Arch          string // dpkg --print-architecture (ou a do Go, sem dpkg)
	Codename      string // jammy, noble, resolute, ... (vazio no Fedora)
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
	// O sudo costuma limpar XDG_CURRENT_DESKTOP (env_reset), então confere
	// pelo binário do GNOME Shell.
	if _, err := exec.LookPath("gnome-shell"); err != nil {
		return nil, fmt.Errorf("%s sem o GNOME Shell não é suportado (só com GNOME)", info.PrettyName)
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
	switch info.ID {
	case "ubuntu":
		info.Familia = Ubuntu
		return fromUbuntu(info, rel)
	case "fedora":
		info.Familia = Fedora
		return fromFedora(info, rel)
	}
	return nil, fmt.Errorf("distribuição não suportada: %s (só o Ubuntu e o Fedora Workstation)", info.PrettyName)
}

func fromUbuntu(info *Info, rel map[string]string) (*Info, error) {
	if info.Codename == "" {
		info.Codename = rel["VERSION_CODENAME"]
	}
	for _, v := range ubuntuSuportadas {
		info.Testadas = append(info.Testadas, v.Versao)
	}
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

func fromFedora(info *Info, rel map[string]string) (*Info, error) {
	if rel["VARIANT_ID"] != "workstation" {
		return nil, fmt.Errorf("%s não é suportado: só o Fedora Workstation (não as variantes atômicas como Silverblue e Kinoite, os spins nem o Server)", info.PrettyName)
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
	if i.Codename == "" {
		return i.PrettyName
	}
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
