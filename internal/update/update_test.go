package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewer(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"v1.6.0", "v1.5.0", true},
		{"v1.5.1", "v1.5.0", true},
		{"v2.0.0", "v1.99.99", true},
		{"v1.10.0", "v1.9.0", true},
		{"v1.5.0", "v1.5.0", false},
		{"v1.4.9", "v1.5.0", false},
		{"v1.6.0", "dev", false},
		{"lixo", "v1.5.0", false},
	}
	for _, tt := range tests {
		if got := Newer(tt.a, tt.b); got != tt.want {
			t.Errorf("Newer(%q, %q) = %v, esperado %v", tt.a, tt.b, got, tt.want)
		}
	}
}

type entrada struct {
	nome, conteudo string
	tipo           byte
}

func montarPacote(t *testing.T, entradas []entrada) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entradas {
		h := &tar.Header{Name: e.nome, Mode: 0o644, Size: int64(len(e.conteudo)), Typeflag: e.tipo}
		if e.tipo == tar.TypeSymlink {
			h.Linkname, h.Size = "/etc/passwd", 0
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if e.tipo == tar.TypeReg {
			if _, err := tw.Write([]byte(e.conteudo)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

var pacoteTeste = []entrada{
	{"post-install/post-install", "binário novo", tar.TypeReg},
	{"post-install/README.md", "readme novo", tar.TypeReg},
	{"post-install/OVPN/README.md", "ovpn readme novo", tar.TypeReg},
	{"post-install/GNOME/extensoes.txt", "extensões novas", tar.TypeReg},
	{"post-install/CINNAMON/fixados.txt", "fixados novos", tar.TypeReg},
	{"post-install/Wallpapers/novo.jpg", "jpg", tar.TypeReg},
	// Nada disto pode ser escrito:
	{"post-install/OVPN/empresa.ovpn", "ovpn falso", tar.TypeReg},
	{"post-install/logs/x.log", "log falso", tar.TypeReg},
	{"post-install/../fora.txt", "fuga", tar.TypeReg},
	{"/etc/cron.d/x", "absoluto", tar.TypeReg},
	{"post-install/GNOME/link", "", tar.TypeSymlink},
}

func TestExtrair(t *testing.T) {
	dir := t.TempDir()
	tgz := filepath.Join(dir, "p.tar.gz")
	if err := os.WriteFile(tgz, montarPacote(t, pacoteTeste), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := extrair(tgz, filepath.Join(dir, "x"), true)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"post-install", "README.md", "OVPN/README.md", "GNOME/extensoes.txt", "CINNAMON/fixados.txt", "Wallpapers/novo.jpg"}
	if len(got) != len(want) {
		t.Errorf("extrair = %v, esperado só %v", got, want)
	}
	for _, w := range want {
		if _, ok := got[w]; !ok {
			t.Errorf("faltou %s", w)
		}
	}

	// Binário solto, fora do pacote: só o binário.
	got, err = extrair(tgz, filepath.Join(dir, "y"), false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["post-install"]; len(got) != 1 || !ok {
		t.Errorf("extrair sem pacote completo = %v", got)
	}
}

func TestApply(t *testing.T) {
	tgz := montarPacote(t, pacoteTeste)
	soma := sha256.Sum256(tgz)
	hash := hex.EncodeToString(soma[:])

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pacote":
			w.Write(tgz)
		case "/sha256":
			w.Write([]byte(hash + "  " + pacote + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	antigo := client
	client = srv.Client()
	defer func() { client = antigo }()

	base := t.TempDir()
	for nome, conteudo := range map[string]string{
		"post-install":        "binário velho",
		"GNOME/extensoes.txt": "extensões velhas",
		"OVPN/empresa.ovpn":   "meu perfil",
		"logs/setup.log":      "meu log",
	} {
		p := filepath.Join(base, nome)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(conteudo), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	rel := &Release{Tag: "v9.9.9", url: srv.URL + "/pacote", sumURL: srv.URL + "/sha256", digest: hash}
	if err := Apply(context.Background(), rel, filepath.Join(base, "post-install")); err != nil {
		t.Fatal(err)
	}
	for nome, want := range map[string]string{
		"post-install":        "binário novo",
		"GNOME/extensoes.txt": "extensões novas",
		"Wallpapers/novo.jpg": "jpg",
		"OVPN/empresa.ovpn":   "meu perfil",
		"logs/setup.log":      "meu log",
	} {
		got, err := os.ReadFile(filepath.Join(base, nome))
		if err != nil || string(got) != want {
			t.Errorf("%s = %q (%v), esperado %q", nome, got, err, want)
		}
	}
	if info, _ := os.Stat(filepath.Join(base, "post-install")); info.Mode().Perm() != 0o755 {
		t.Errorf("binário com permissão %o", info.Mode().Perm())
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(base), "fora.txt")); err == nil {
		t.Error("entrada com ../ escreveu fora da pasta")
	}
	if sobras, _ := filepath.Glob(filepath.Join(base, ".post-install-update-*")); len(sobras) > 0 {
		t.Errorf("diretório temporário não removido: %v", sobras)
	}

	// SHA-256 que não confere: nada é alterado.
	rel.digest = "00" + hash[2:]
	os.WriteFile(filepath.Join(base, "post-install"), []byte("binário velho"), 0o755)
	if err := Apply(context.Background(), rel, filepath.Join(base, "post-install")); err == nil {
		t.Fatal("Apply aceitou um pacote com SHA-256 diferente")
	}
	if got, _ := os.ReadFile(filepath.Join(base, "post-install")); string(got) != "binário velho" {
		t.Errorf("binário alterado mesmo com SHA-256 errado: %q", got)
	}
}
