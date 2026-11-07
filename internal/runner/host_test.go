package runner

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/vinicgobbi/post-install/internal/runlog"
)

type hostTeste struct{ fs fstest.MapFS }

func (h hostTeste) Consultar(name string, args []string) ([]byte, []byte, error) {
	if name == "dpkg" {
		return []byte("instalado\n"), nil, nil
	}
	return nil, []byte("não encontrado\n"), errors.New("exit status 1")
}

func (h hostTeste) LookPath(name string) (string, error) {
	if name == "git" {
		return "/usr/bin/git", nil
	}
	return "", errors.New("não encontrado")
}

func (h hostTeste) FS() fs.FS { return h.fs }

func (h hostTeste) Transport() http.RoundTripper {
	return roundTrip(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok " + req.URL.String())), Request: req}, nil
	})
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestHost(t *testing.T) {
	log, _ := runlog.New("", "")
	r := &Runner{Ctx: context.Background(), Log: log, Host: hostTeste{fstest.MapFS{
		"etc/subuid":               {Data: []byte("ana:100000:65536\n")},
		"opt/OVPN/a.ovpn":          {Data: []byte("client\n")},
		"opt/OVPN/b.ovpn":          {Data: []byte("client\n")},
		"usr/share/apps/x.desktop": {Data: []byte("[Desktop Entry]\n")},
	}}}

	if !r.Probe("dpkg", "-s", "x") || r.Probe("snap", "list") {
		t.Error("Probe não seguiu o Host")
	}
	if out, err := r.Output("dpkg", "-s", "x"); err != nil || out != "instalado\n" {
		t.Errorf("Output = %q, %v", out, err)
	}
	if _, err := r.Output("snap", "list"); err == nil || !strings.Contains(err.Error(), "snap list") {
		t.Errorf("erro do Output sem o comando: %v", err)
	}
	if !r.Exists("git") || r.Exists("zsh") {
		t.Error("Exists não seguiu o Host")
	}
	if b, err := r.ReadFile("/etc/subuid"); err != nil || string(b) != "ana:100000:65536\n" {
		t.Errorf("ReadFile = %q, %v", b, err)
	}
	if _, err := r.ReadFile("/etc/subgid"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("arquivo ausente deveria dar ErrNotExist: %v", err)
	}
	if !r.FileExists("/opt/OVPN") || r.FileExists("/nao/existe") {
		t.Error("FileExists não seguiu o Host")
	}
	if g, _ := r.Glob("/opt/OVPN/*.ovpn"); !slices.Equal(g, []string{"/opt/OVPN/a.ovpn", "/opt/OVPN/b.ovpn"}) {
		t.Errorf("Glob = %v", g)
	}
	var vistos []string
	r.WalkDir("/usr/share/apps", func(p string, d fs.DirEntry, err error) error {
		vistos = append(vistos, p)
		return nil
	})
	if !slices.Equal(vistos, []string{"/usr/share/apps", "/usr/share/apps/x.desktop"}) {
		t.Errorf("WalkDir = %v", vistos)
	}
	if b, err := r.Get("https://exemplo.test/a", time.Second); err != nil || string(b) != "ok https://exemplo.test/a" {
		t.Errorf("Get = %q, %v", b, err)
	}
}

func TestCaminhoFS(t *testing.T) {
	for in, want := range map[string]string{"/": ".", "/etc/subuid": "etc/subuid", "/a/../b/": "b"} {
		if got := caminhoFS(in); got != want {
			t.Errorf("caminhoFS(%q) = %q, esperado %q", in, got, want)
		}
	}
}
