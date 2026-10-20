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
		name        string
		osRelease   string
		upstream    string
		family      string
		codename    string
		version     string
		msSupported bool
		warnings    int
		wantErr     bool
	}{
		{name: "fedora", osRelease: `ID=fedora
VERSION_ID=42
PRETTY_NAME="Fedora Linux 42"`, family: "fedora"},
		{name: "alma", osRelease: `ID="almalinux"
ID_LIKE="rhel centos fedora"
VERSION_ID="9.4"`, family: "rhel"},
		{name: "arch", osRelease: "ID=arch", family: "arch"},
		{name: "manjaro nao suportado", osRelease: "ID=manjaro\nID_LIKE=arch", wantErr: true},
		{name: "ubuntu noble", osRelease: `ID=ubuntu
ID_LIKE=debian
VERSION_CODENAME=noble
UBUNTU_CODENAME=noble`, family: "debian", codename: "noble", version: "24.04", msSupported: true},
		{name: "ubuntu futuro usa fallback", osRelease: `ID=ubuntu
UBUNTU_CODENAME=zesty2`, family: "debian", codename: "zesty2", version: "26.04", msSupported: true, warnings: 1},
		{name: "ubuntu resolute", osRelease: `ID=ubuntu
ID_LIKE=debian
VERSION_CODENAME=resolute
UBUNTU_CODENAME=resolute`, family: "debian", codename: "resolute", version: "26.04", msSupported: true},
		{name: "mint usa upstream", osRelease: `ID=linuxmint
ID_LIKE="ubuntu debian"
VERSION_CODENAME=wilma`, upstream: "DISTRIB_CODENAME=noble", family: "debian", codename: "noble", version: "24.04", msSupported: true},
		{name: "debian bookworm", osRelease: `ID=debian
VERSION_ID="12"
VERSION_CODENAME=bookworm`, family: "debian", codename: "bookworm", version: "12", msSupported: true},
		{name: "debian trixie", osRelease: `ID=debian
VERSION_ID="13"
VERSION_CODENAME=trixie`, family: "debian", codename: "trixie", version: "13", msSupported: true},
		{name: "debian forky sem repo MS", osRelease: `ID=debian
VERSION_ID="14"
VERSION_CODENAME=forky`, family: "debian", codename: "forky", version: "14"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := fromRelease(kv(tt.osRelease), kv(tt.upstream))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("esperava erro, veio %+v", info)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if info.Family != tt.family || info.BaseCodename != tt.codename ||
				info.DistroVersion != tt.version || info.MSRepoSupported != tt.msSupported ||
				len(info.Warnings) != tt.warnings {
				t.Errorf("resultado inesperado: %+v", info)
			}
		})
	}
}

func TestRHELVersion(t *testing.T) {
	info, err := fromRelease(kv(`ID=rocky
ID_LIKE="rhel centos fedora"
VERSION_ID="9.4"`), nil)
	if err != nil || info.RHELVersion != "9" {
		t.Fatalf("got %+v, %v", info, err)
	}
}
