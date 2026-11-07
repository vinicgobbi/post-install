package modules

// Testes golden do --dry-run: cada cenário descreve uma máquina (o que está
// instalado, os arquivos e o que a rede responde) e o teste roda todos os
// módulos em dry-run contra ela, comparando o que cada um faria com
// testdata/golden/<cenário>.txt. Qualquer mudança no que um módulo faz
// aparece como diferença nesses arquivos.
//
// Para regenerar depois de uma mudança intencional (e revisar o diff):
//
//	go test ./internal/modules -run TestGolden -update

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/vinicgobbi/post-install/internal/runlog"
	"github.com/vinicgobbi/post-install/internal/runner"
	"github.com/vinicgobbi/post-install/internal/sysinfo"
)

var atualizarGolden = flag.Bool("update", false, "regrava os arquivos de testdata/golden")

// ---- Máquina falsa ----

type hostFalso struct {
	arquivos fstest.MapFS
	binarios map[string]bool
	// comandos: chave (veja chaveComando) -> stdout. Ausente = o comando
	// falha (código de saída 1).
	comandos map[string]string
	// http: URL -> corpo (HTTP 200). Ausente = HTTP 404.
	http map[string]string

	mu          sync.Mutex
	semResposta map[string]bool
}

// chaveComando identifica uma consulta. Os scripts rodados como o usuário
// (su - usuario -c) viram "[usuario] <script>", sem o trecho fixo que o
// runner põe antes (o da sessão D-Bus, que termina no primeiro "fi").
func chaveComando(name string, args []string) string {
	if name == "su" && len(args) == 4 && args[0] == "-" && args[2] == "-c" {
		script := args[3]
		if _, depois, ok := strings.Cut(script, "fi\n"); ok {
			script = depois
		}
		return "[" + args[1] + "] " + strings.TrimSpace(script)
	}
	return strings.Join(append([]string{name}, args...), " ")
}

func (h *hostFalso) anotar(s string) {
	h.mu.Lock()
	h.semResposta[s] = true
	h.mu.Unlock()
}

func (h *hostFalso) Consultar(name string, args []string) ([]byte, []byte, error) {
	k := chaveComando(name, args)
	if out, ok := h.comandos[k]; ok {
		return []byte(out), nil, nil
	}
	h.anotar("comando: " + k)
	return nil, nil, errors.New("exit status 1")
}

func (h *hostFalso) LookPath(name string) (string, error) {
	if h.binarios[name] {
		return "/usr/bin/" + name, nil
	}
	return "", fmt.Errorf("exec: %q: executable file not found in $PATH", name)
}

func (h *hostFalso) FS() fs.FS { return h.arquivos }

func (h *hostFalso) Transport() http.RoundTripper { return h }

func (h *hostFalso) RoundTrip(req *http.Request) (*http.Response, error) {
	u := req.URL.String()
	corpo, ok := h.http[u]
	status := http.StatusOK
	if !ok {
		h.anotar("GET " + u)
		status, corpo = http.StatusNotFound, "não encontrado"
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(corpo)),
		Header:     http.Header{},
		Request:    req,
	}, nil
}

// ---- Cenários ----

type cenario struct {
	nome      string
	sys       sysinfo.Info
	host      *hostFalso
	forcar    []string
	wallpaper string
}

const (
	dirPacote = "/opt/post-install"
	homeAna   = "/home/ana"
	sha       = "0000000000000000000000000000000000000000000000000000000000000000"
)

var ubuntus = map[string]sysinfo.Info{
	"22.04": {Familia: sysinfo.Ubuntu, ID: "ubuntu", PrettyName: "Ubuntu 22.04.5 LTS", VersionID: "22.04", Codename: "jammy", MSRepoVersion: "22.04", Arch: "amd64"},
	"24.04": {Familia: sysinfo.Ubuntu, ID: "ubuntu", PrettyName: "Ubuntu 24.04.3 LTS", VersionID: "24.04", Codename: "noble", MSRepoVersion: "24.04", Arch: "amd64"},
	"26.04": {Familia: sysinfo.Ubuntu, ID: "ubuntu", PrettyName: "Ubuntu 26.04.1 LTS", VersionID: "26.04", Codename: "resolute", MSRepoVersion: "26.04", Arch: "amd64"},
}

var gnomeShell = map[string]string{"22.04": "42", "24.04": "46", "26.04": "50"}

func arq(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s), Mode: 0o644} }

// ubuntuLimpo é um Ubuntu Desktop recém-instalado: GNOME, snaps do Firefox
// e do Thunderbird, sem zsh, flatpak, PHP etc., e com rede.
func ubuntuLimpo(t *testing.T, versao string) cenario {
	sys := ubuntus[versao]
	shell := gnomeShell[versao]
	h := &hostFalso{
		arquivos: fstest.MapFS{
			"etc/subuid":   arq("ana:100000:65536\n"),
			"etc/subgid":   arq("ana:100000:65536\n"),
			"proc/cpuinfo": arq("flags\t\t: fpu vme vmx sse sse2\n"),
			"usr/share/gnome-shell/extensions/ubuntu-dock@ubuntu.com/metadata.json": arq("{}"),
			"usr/share/applications/org.gnome.Nautilus.desktop":                     arq("[Desktop Entry]\nType=Application\nCategories=GNOME;GTK;Utility;Core;FileManager;\n"),
			"usr/share/applications/org.gnome.Settings.desktop":                     arq("[Desktop Entry]\nType=Application\nCategories=GNOME;GTK;Settings;\n"),
			"usr/share/applications/oculto.desktop":                                 arq("[Desktop Entry]\nType=Application\nNoDisplay=true\n"),
			"var/lib/snapd/desktop/applications/firefox_firefox.desktop":            arq("[Desktop Entry]\nType=Application\nCategories=Network;WebBrowser;\n"),
			"usr/share/desktop-directories/Game.directory":                          arq("[Desktop Entry]\nName=Games\n"),
		},
		binarios: map[string]bool{"gnome-shell": true, "gnome-extensions": true, "snap": true, "add-apt-repository": true},
		comandos: map[string]string{
			"getent -s files passwd ana":         "ana:x:1000:1000:Ana:/home/ana:/bin/bash\n",
			"getent passwd ana":                  "ana:x:1000:1000:Ana:/home/ana:/bin/bash\n",
			"getent group plugdev":               "plugdev:x:46:\n",
			"getent group kvm":                   "kvm:x:993:\n",
			"id -u ana":                          "1000\n",
			"snap list firefox":                  "firefox 140.0 6000 latest/stable mozilla** -\n",
			"snap list thunderbird":              "thunderbird 140.0 800 latest/stable canonical** -\n",
			"nmcli -g NAME,TYPE connection show": "Wired connection 1:802-3-ethernet\n",
			"gnome-shell --version":              "GNOME Shell " + shell + ".1\n",
			"[ana] gsettings get org.gnome.shell enabled-extensions":  "['ubuntu-dock@ubuntu.com', 'ding@rastersoft.com']\n",
			"[ana] gsettings get org.gnome.shell disabled-extensions": "@as []\n",
		},
		http: map[string]string{
			"http://mirrors.ubuntu.com/mirrors.txt":                                  "http://br.archive.ubuntu.com/ubuntu/\n",
			"https://ifconfig.co/country":                                            "Brazil\n",
			"https://launchpad.net/ubuntu/+archivemirrors":                           "<tr class=\"head\"><th colspan=\"2\">Brazil</th></tr>\n<tr><td><a href=\"https://mirror.exemplo.br/ubuntu/\">x</a></td></tr>\n<tr class=\"head\"><th colspan=\"2\">Canada</th></tr>\n",
			"https://launchpad.net/ubuntu/+archivemirrors-rss":                       "<link>http://mirror.exemplo.ca/ubuntu/</link>\n",
			"http://br.archive.ubuntu.com/ubuntu/dists/" + sys.Codename + "/Release": "Origin: Ubuntu\n",

			"https://gitea.com/api/v1/repos/gitea/tea/releases/latest":                               `{"tag_name":"v0.11.1"}`,
			"https://gitea.com/gitea/tea/releases/download/v0.11.1/tea-0.11.1-linux-amd64.xz.sha256": sha + "  tea-0.11.1-linux-amd64.xz\n",
			"https://api.github.com/repos/git-ecosystem/git-credential-manager/releases/latest": `{"assets":[
				{"name":"gcm-linux_amd64.2.6.1.tar.gz","browser_download_url":"https://github.com/git-ecosystem/git-credential-manager/releases/download/v2.6.1/gcm-linux_amd64.2.6.1.tar.gz","digest":"sha256:` + sha + `"},
				{"name":"gcm-linux-x64-2.6.1.deb","browser_download_url":"https://github.com/git-ecosystem/git-credential-manager/releases/download/v2.6.1/gcm-linux-x64-2.6.1.deb","digest":"sha256:` + sha + `"}]}`,
			"https://api.github.com/repos/vinicgobbi/deploy-tools/releases/latest": `{"assets":[
				{"name":"deploy-dev-linux-amd64","browser_download_url":"https://github.com/vinicgobbi/deploy-tools/releases/download/v1.0.0/deploy-dev-linux-amd64","digest":"sha256:` + sha + `"},
				{"name":"deploy-dev-linux-amd64.sha256","browser_download_url":"https://github.com/vinicgobbi/deploy-tools/releases/download/v1.0.0/deploy-dev-linux-amd64.sha256","digest":""}]}`,
			"https://extensions.gnome.org/extension-info/?uuid=compativel%40exemplo.test":   `{"shell_version_map":{"` + shell + `":{"pk":101},"40":{"pk":11}}}`,
			"https://extensions.gnome.org/extension-info/?uuid=incompativel%40exemplo.test": `{"shell_version_map":{"40":{"pk":22},"41":{"pk":23}}}`,
		},
		semResposta: map[string]bool{},
	}
	if versao == "22.04" {
		h.arquivos["etc/apt/sources.list"] = arq("deb http://archive.ubuntu.com/ubuntu/ jammy main restricted\ndeb http://security.ubuntu.com/ubuntu/ jammy-security main restricted\n")
		h.arquivos["usr/share/applications/org.gnome.Terminal.desktop"] = arq("[Desktop Entry]\nType=Application\nCategories=GNOME;GTK;System;TerminalEmulator;\n")
	} else {
		h.arquivos["etc/apt/sources.list"] = arq("# Ubuntu sources have moved to /etc/apt/sources.list.d/ubuntu.sources\n")
		h.arquivos["etc/apt/sources.list.d/ubuntu.sources"] = arq("Types: deb\nURIs: http://archive.ubuntu.com/ubuntu/\nSuites: " + sys.Codename + " " + sys.Codename + "-updates\nComponents: main restricted\n")
		h.comandos["apt-cache show libfuse2t64"] = "Package: libfuse2t64\n"
		terminal := "org.gnome.Terminal.desktop"
		if versao == "26.04" {
			terminal = "org.gnome.Ptyxis.desktop"
		}
		h.arquivos["usr/share/applications/"+terminal] = arq("[Desktop Entry]\nType=Application\nCategories=GNOME;GTK;System;TerminalEmulator;\n")
	}
	for _, nome := range []string{"extensoes.txt", "extensoes.dconf", "atalhos.dconf", "favoritos.txt", "pastas.txt"} {
		b, err := os.ReadFile(filepath.Join("testdata/gnome", nome))
		if err != nil {
			t.Fatal(err)
		}
		h.arquivos[strings.TrimPrefix(dirPacote, "/")+"/GNOME/"+nome] = arq(string(b))
	}
	return cenario{nome: "ubuntu-" + versao + "-limpo", sys: sys, host: h, wallpaper: dirPacote + "/Wallpapers/paisagem.jpg"}
}

// ubuntuConfigurado é um Ubuntu 26.04 onde o post-install já rodou e com os
// casos menos comuns: conta de domínio sem faixa de subuid, sem rede para os
// mirrors e o tea, perfil OpenVPN legível por todos, CPU sem VT-x, extensão
// incompatível forçada.
func ubuntuConfigurado(t *testing.T) cenario {
	c := ubuntuLimpo(t, "26.04")
	c.nome = "ubuntu-26.04-configurado"
	c.forcar = []string{"incompativel@exemplo.test"}
	c.wallpaper = ""
	h := c.host
	for _, b := range []string{"batcat", "zsh", "flatpak", "make", "curl", "php"} {
		h.binarios[b] = true
	}
	delete(h.binarios, "add-apt-repository")
	for _, k := range []string{"getent -s files passwd ana", "snap list firefox", "snap list thunderbird", "getent group plugdev"} {
		delete(h.comandos, k)
	}
	h.comandos["getent group libvirt"] = "libvirt:x:130:ana\n"
	h.comandos["dpkg -s network-manager-openvpn"] = "Status: install ok installed\n"
	h.comandos["flatpak info "+gradiaFlatpak] = "Gradia\n"
	h.comandos[`php -r echo PHP_MAJOR_VERSION.".".PHP_MINOR_VERSION;`] = "8.4"
	h.comandos["nmcli -g NAME,TYPE connection show"] = "Wired connection 1:802-3-ethernet\nempresa:vpn\nescritorio:802-11-wireless\n"
	h.comandos["[ana] gsettings get org.gnome.shell enabled-extensions"] = "['ubuntu-dock@ubuntu.com', 'compativel@exemplo.test']\n"
	h.comandos["[ana] gsettings get org.gnome.shell disabled-extensions"] = "['ding@rastersoft.com']\n"
	for _, u := range []string{"http://mirrors.ubuntu.com/mirrors.txt", "https://ifconfig.co/country",
		"https://launchpad.net/ubuntu/+archivemirrors", "https://launchpad.net/ubuntu/+archivemirrors-rss",
		"https://gitea.com/api/v1/repos/gitea/tea/releases/latest"} {
		delete(h.http, u)
	}
	h.arquivos["etc/subuid"] = arq("outro:100000:65536\n")
	delete(h.arquivos, "etc/subgid")
	h.arquivos["proc/cpuinfo"] = arq("flags\t\t: fpu vme sse sse2\n")
	h.arquivos["usr/lib/udev/rules.d/"+regraSolaarNome] = arq("# regra do pacote\n")
	h.arquivos["usr/share/applications/firefox.desktop"] = arq("[Desktop Entry]\nType=Application\nCategories=Network;WebBrowser;\n")
	delete(h.arquivos, "var/lib/snapd/desktop/applications/firefox_firefox.desktop")
	ovpn := strings.TrimPrefix(dirPacote, "/") + "/OVPN/"
	h.arquivos[ovpn+"empresa.ovpn"] = arq("client\nremote vpn.exemplo.test 1194\ndhcp-option DNS 10.0.0.1\ndhcp-option DOMAIN corp.exemplo.test\n")
	h.arquivos[ovpn+"escritorio.ovpn"] = &fstest.MapFile{Data: []byte("client\n"), Mode: 0o600}
	return c
}

func cenarios(t *testing.T) []cenario {
	return []cenario{
		ubuntuLimpo(t, "22.04"),
		ubuntuLimpo(t, "24.04"),
		ubuntuLimpo(t, "26.04"),
		ubuntuConfigurado(t),
	}
}

// ---- Execução ----

// normalizar troca valores que mudam sem alterar o comportamento dos módulos
// (commits e versões fixados, atualizados pelo workflow pins.yml) por nomes
// fixos, para esses PRs não exigirem regravar os golden.
func normalizar(s string) string {
	return strings.NewReplacer(
		dotfilesCommit, "<dotfilesCommit>",
		vscodeNautilusCommit, "<vscodeNautilusCommit>",
		copypathCommit, "<copypathCommit>",
		"Solaar "+solaarTag, "Solaar <solaarTag>",
		regraSolaarDestino+" ("+strconv.Itoa(len(regraSolaar))+" bytes)", regraSolaarDestino+" (<regraSolaar> bytes)",
	).Replace(s)
}

func rodarCenario(t *testing.T, c cenario) string {
	var (
		out strings.Builder
		mu  sync.Mutex
	)
	log, err := runlog.New("", "")
	if err != nil {
		t.Fatal(err)
	}
	log.SetSink(func(e runlog.Entry) {
		mu.Lock()
		fmt.Fprintf(&out, "[%s] %s\n", e.Level, e.Text)
		mu.Unlock()
	})
	sys := c.sys
	ctx := &Ctx{
		Runner: &runner.Runner{
			Ctx:     context.Background(),
			Log:     log,
			User:    "ana",
			DryRun:  true,
			WorkDir: "/tmp/post-install-teste",
			Host:    c.host,
		},
		Sys:             &sys,
		OVPNDir:         dirPacote + "/OVPN",
		GNOMEDir:        dirPacote + "/GNOME",
		Wallpaper:       c.wallpaper,
		ForcarExtensoes: c.forcar,
	}

	fmt.Fprintf(&out, "# Cenário %s: %s, usuário ana, dry-run\n", c.nome, sys.PrettyName)
	for _, m := range Disponiveis(&sys) {
		mu.Lock()
		fmt.Fprintf(&out, "\n== %s (%s)\n", m.ID, m.Title)
		mu.Unlock()
		err := m.Run(ctx)
		mu.Lock()
		if err != nil {
			fmt.Fprintf(&out, "=> ERRO: %v\n", err)
		}
		mu.Unlock()
	}

	pend := make([]string, 0, len(c.host.semResposta))
	for k := range c.host.semResposta {
		pend = append(pend, k)
	}
	slices.Sort(pend)
	out.WriteString("\n== Consultas que o cenário não responde (falham na máquina falsa)\n")
	for _, p := range pend {
		out.WriteString(p + "\n")
	}
	return normalizar(out.String())
}

func TestGolden(t *testing.T) {
	for _, c := range cenarios(t) {
		t.Run(c.nome, func(t *testing.T) {
			got := rodarCenario(t, c)
			arquivo := filepath.Join("testdata", "golden", c.nome+".txt")
			if *atualizarGolden {
				if err := os.MkdirAll(filepath.Dir(arquivo), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(arquivo, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(arquivo)
			if err != nil {
				t.Fatalf("%v (rode com -update para criar)", err)
			}
			if got != string(want) {
				t.Errorf("o dry-run do cenário %s mudou em relação a %s.\n"+
					"Se a mudança for intencional, rode: go test ./internal/modules -run TestGolden -update\n%s",
					c.nome, arquivo, diffLinhas(string(want), got))
			}
		})
	}
}

// diffLinhas mostra as primeiras linhas que diferem (sem dependências
// externas: basta para localizar a mudança).
func diffLinhas(want, got string) string {
	a, b := strings.Split(want, "\n"), strings.Split(got, "\n")
	var out strings.Builder
	n := 0
	for i := 0; i < max(len(a), len(b)) && n < 20; i++ {
		var x, y string
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			fmt.Fprintf(&out, "linha %d:\n  - %s\n  + %s\n", i+1, x, y)
			n++
		}
	}
	return out.String()
}
