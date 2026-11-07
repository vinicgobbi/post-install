package sysinfo

import (
	"bufio"
	"strings"
	"testing"
)

func kv(s string) map[string]string {
	return parseKV(bufio.NewScanner(strings.NewReader(s)))
}

func TestFromRelease(t *testing.T) {
	tests := []struct {
		name       string
		osRelease  string
		familia    Familia
		codename   string
		msVersion  string
		warnings   int
		naoTestada bool
		wantErr    bool
	}{
		{name: "ubuntu jammy", osRelease: `ID=ubuntu
ID_LIKE=debian
VERSION_ID="22.04"
VERSION_CODENAME=jammy
UBUNTU_CODENAME=jammy`, familia: Ubuntu, codename: "jammy", msVersion: "22.04"},
		{name: "ubuntu noble", osRelease: `ID=ubuntu
ID_LIKE=debian
VERSION_ID="24.04"
VERSION_CODENAME=noble
UBUNTU_CODENAME=noble`, familia: Ubuntu, codename: "noble", msVersion: "24.04"},
		{name: "ubuntu resolute", osRelease: `ID=ubuntu
VERSION_ID="26.04"
UBUNTU_CODENAME=resolute`, familia: Ubuntu, codename: "resolute", msVersion: "26.04"},
		{name: "ubuntu futuro usa fallback e pede confirmação", osRelease: `ID=ubuntu
VERSION_ID="26.10"
UBUNTU_CODENAME=zesty2`, familia: Ubuntu, codename: "zesty2", msVersion: "26.04", warnings: 2, naoTestada: true},
		{name: "ubuntu focal nao suportado", osRelease: `ID=ubuntu
VERSION_ID="20.04"
UBUNTU_CODENAME=focal`, wantErr: true},
		{name: "ubuntu interina antiga nao suportada", osRelease: `ID=ubuntu
VERSION_ID="25.04"
UBUNTU_CODENAME=plucky`, wantErr: true},
		{name: "mint nao suportado", osRelease: `ID=linuxmint
ID_LIKE="ubuntu debian"
UBUNTU_CODENAME=noble`, wantErr: true},
		{name: "debian nao suportado", osRelease: `ID=debian
VERSION_ID="12"
VERSION_CODENAME=bookworm`, wantErr: true},
		{name: "fedora workstation", osRelease: `ID=fedora
VERSION_ID=44
VARIANT_ID=workstation
PRETTY_NAME="Fedora Linux 44 (Workstation Edition)"`, familia: Fedora, msVersion: "10"},
		{name: "fedora futuro pede confirmação", osRelease: "ID=fedora\nVERSION_ID=99\nVARIANT_ID=workstation", familia: Fedora, msVersion: "10", warnings: 1, naoTestada: true},
		{name: "fedora fora de manutenção", osRelease: "ID=fedora\nVERSION_ID=40\nVARIANT_ID=workstation", wantErr: true},
		{name: "fedora sem variante", osRelease: "ID=fedora\nVERSION_ID=44", wantErr: true},
		{name: "fedora silverblue", osRelease: "ID=fedora\nVERSION_ID=44\nVARIANT_ID=silverblue", wantErr: true},
		{name: "fedora kde", osRelease: "ID=fedora\nVERSION_ID=44\nVARIANT_ID=kde", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := fromRelease(kv(tt.osRelease))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("esperava erro, veio %+v", info)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if info.Familia != tt.familia || info.Codename != tt.codename || info.MSRepoVersion != tt.msVersion ||
				len(info.Warnings) != tt.warnings || info.NaoTestada != tt.naoTestada {
				t.Errorf("resultado inesperado: %+v", info)
			}
		})
	}
}

// As listas de versoes.go precisam estar em ordem crescente: a última é a
// referência para "mais nova que as testadas".
func TestVersoesEmOrdem(t *testing.T) {
	for i := 1; i < len(ubuntuSuportadas); i++ {
		if compararVersoes(ubuntuSuportadas[i-1].Versao, ubuntuSuportadas[i].Versao) >= 0 {
			t.Errorf("ubuntuSuportadas fora de ordem: %v", ubuntuSuportadas)
		}
	}
	for i := 1; i < len(fedoraSuportadas); i++ {
		if compararVersoes(fedoraSuportadas[i-1].Versao, fedoraSuportadas[i].Versao) >= 0 {
			t.Errorf("fedoraSuportadas fora de ordem: %v", fedoraSuportadas)
		}
	}
	if len(ubuntuSuportadas) == 0 || len(fedoraSuportadas) == 0 {
		t.Error("lista de versões vazia")
	}
}

func TestJuntar(t *testing.T) {
	for in, want := range map[string]string{"": "", "a": "a", "a,b": "a e b", "a,b,c": "a, b e c"} {
		var itens []string
		if in != "" {
			itens = strings.Split(in, ",")
		}
		if got := juntar(itens); got != want {
			t.Errorf("juntar(%q) = %q, esperado %q", in, got, want)
		}
	}
}

func TestSummary(t *testing.T) {
	if s := (&Info{PrettyName: "Ubuntu 26.04 LTS", Codename: "resolute"}).Summary(); s != "Ubuntu 26.04 LTS (resolute)" {
		t.Errorf("Summary do Ubuntu = %q", s)
	}
	if s := (&Info{PrettyName: "Fedora Linux 44 (Workstation Edition)"}).Summary(); s != "Fedora Linux 44 (Workstation Edition)" {
		t.Errorf("Summary do Fedora = %q", s)
	}
}
