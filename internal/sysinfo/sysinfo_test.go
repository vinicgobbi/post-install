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
		name      string
		osRelease string
		codename  string
		msVersion string
		warnings  int
		wantErr   bool
	}{
		{name: "ubuntu jammy", osRelease: `ID=ubuntu
ID_LIKE=debian
VERSION_ID="22.04"
VERSION_CODENAME=jammy
UBUNTU_CODENAME=jammy`, codename: "jammy", msVersion: "22.04"},
		{name: "ubuntu noble", osRelease: `ID=ubuntu
ID_LIKE=debian
VERSION_ID="24.04"
VERSION_CODENAME=noble
UBUNTU_CODENAME=noble`, codename: "noble", msVersion: "24.04"},
		{name: "ubuntu resolute", osRelease: `ID=ubuntu
VERSION_ID="26.04"
UBUNTU_CODENAME=resolute`, codename: "resolute", msVersion: "26.04"},
		{name: "ubuntu futuro usa fallback", osRelease: `ID=ubuntu
VERSION_ID="26.10"
UBUNTU_CODENAME=zesty2`, codename: "zesty2", msVersion: "26.04", warnings: 1},
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
		{name: "fedora nao suportado", osRelease: "ID=fedora\nVERSION_ID=42", wantErr: true},
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
			if info.Codename != tt.codename || info.MSRepoVersion != tt.msVersion || len(info.Warnings) != tt.warnings {
				t.Errorf("resultado inesperado: %+v", info)
			}
		})
	}
}
