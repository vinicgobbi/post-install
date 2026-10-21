package modules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestIDsUnicos(t *testing.T) {
	vistos := map[string]bool{}
	for _, m := range All() {
		if vistos[m.ID] {
			t.Errorf("ID duplicado: %s", m.ID)
		}
		vistos[m.ID] = true
		if m.Run == nil {
			t.Errorf("%s sem função Run", m.ID)
		}
	}
}

func TestOpcoesDNS(t *testing.T) {
	dns, dom := opcoesDNS(`client
remote vpn.example.com 1194
dhcp-option DNS 10.0.0.1
  dhcp-option DNS 10.0.0.2
dhcp-option DOMAIN corp.local
dhcp-option DOMAIN-SEARCH lab.local
# dhcp-option DNS 9.9.9.9
`)
	if !slices.Equal(dns, []string{"10.0.0.1", "10.0.0.2"}) {
		t.Errorf("dns = %v", dns)
	}
	if !slices.Equal(dom, []string{"corp.local", "lab.local"}) {
		t.Errorf("dominios = %v", dom)
	}
}

func TestMirrorsDoPais(t *testing.T) {
	page := `<tr class="head"><th colspan="2">Argentina</th></tr>
<tr><td><a href="https://ar.example/ubuntu/">x</a></td></tr>
<tr class="head"><th colspan="2">Brazil</th></tr>
<tr><td><a href="http://br1.example/ubuntu/">x</a></td></tr>
<tr><td><a href="https://br2.example/ubuntu/">x</a> <a href="rsync://br2.example/ubuntu/">r</a></td></tr>
<tr class="head"><th colspan="2">Canada</th></tr>
<tr><td><a href="https://ca.example/ubuntu/">x</a></td></tr>`
	got := mirrorsDoPais(page, "Brazil")
	if !slices.Equal(got, []string{"http://br1.example/ubuntu/", "https://br2.example/ubuntu/"}) {
		t.Errorf("got %v", got)
	}
}

func TestUsaFormatoDeb822(t *testing.T) {
	for v, want := range map[string]bool{"20.04": false, "22.04": false, "24.04": false, "24.10": true, "26.04": true, "": true} {
		if got := usaFormatoDeb822(v); got != want {
			t.Errorf("usaFormatoDeb822(%q) = %v, esperado %v", v, got, want)
		}
	}
}

func TestLerListaExtensoes(t *testing.T) {
	ativar, desativar, git, err := lerListaExtensoes("../../GNOME/extensoes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(ativar) != 15 || !slices.Contains(ativar, "gsconnect@andyholmes.github.io") || !slices.Contains(ativar, "ubuntu-dock@ubuntu.com") {
		t.Errorf("ativar = %v", ativar)
	}
	if !slices.Equal(desativar, []string{"ding@rastersoft.com"}) {
		t.Errorf("desativar = %v", desativar)
	}
	if !slices.Contains(ativar, "nowbar@vinicgobbi.dev.br") || git["nowbar@vinicgobbi.dev.br"].Repo != "https://github.com/vinicgobbi/gnome-extension-nowbar.git" ||
		len(git["nowbar@vinicgobbi.dev.br"].Ref) != 40 || len(git) != 1 {
		t.Errorf("git = %v", git)
	}
}

func TestParseStrvEMesclar(t *testing.T) {
	if got := parseStrv("['a@x', 'b@y']\n"); !slices.Equal(got, []string{"a@x", "b@y"}) {
		t.Errorf("parseStrv = %v", got)
	}
	if got := parseStrv("@as []"); len(got) != 0 {
		t.Errorf("parseStrv vazio = %v", got)
	}
	got := mesclar([]string{"a", "ding"}, []string{"b", "a"}, []string{"ding"})
	if !slices.Equal(got, []string{"a", "b"}) || strv(got) != "['a', 'b']" {
		t.Errorf("mesclar = %v / %s", got, strv(got))
	}
}

func TestSemControle(t *testing.T) {
	var v struct{ D string }
	if err := json.Unmarshal(semControle([]byte("{\"D\":\"linha1\nlinha2\"}")), &v); err != nil || v.D != "linha1 linha2" {
		t.Errorf("v = %+v, err = %v", v, err)
	}
}

func TestConsultarExtensao(t *testing.T) {
	// Como a API real: caractere de controle cru na descrição e
	// shell_version_map com versões antigas no formato "3.36".
	resp := []byte("{\"description\":\"linha1\nlinha2\",\"shell_version_map\":{\"3.36\":{\"pk\":1},\"45\":{\"pk\":2},\"46\":{\"pk\":3},\"50\":{\"pk\":4}}}")
	get := func(string) ([]byte, error) { return resp, nil }

	info, err := ConsultarExtensao(get, "x@y", "50")
	if err != nil || !info.Compativel || info.versionTag != 4 || info.MaiorShell != "50" {
		t.Errorf("compatível: %+v, %v", info, err)
	}
	// Sem versão para o 51: incompatível, mas aponta para o pacote mais recente.
	info, err = ConsultarExtensao(get, "x@y", "51")
	if err != nil || info.Compativel || info.versionTag != 4 || info.MaiorShell != "50" {
		t.Errorf("incompatível: %+v, %v", info, err)
	}
}

func TestVersaoMaior(t *testing.T) {
	if !versaoMaior("46", "3.38") || !versaoMaior("50", "46") || versaoMaior("3.36", "40") || versaoMaior("46", "46") {
		t.Error("comparação de versões errada")
	}
}

func TestStrvEscapa(t *testing.T) {
	if got := strv([]string{`a'b`, `c\d`}); got != `['a\'b', 'c\\d']` {
		t.Errorf("strv = %s", got)
	}
}

func TestConexoesNM(t *testing.T) {
	saida := "Wired connection 1:802-3-ethernet\nvpn\\:corp:vpn\nempresa:vpn\n"
	got := conexoesNM(saida)
	want := map[string]string{"Wired connection 1": "802-3-ethernet", "vpn:corp": "vpn", "empresa": "vpn"}
	if len(got) != len(want) {
		t.Fatalf("conexoesNM = %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("conexoesNM[%q] = %q, esperado %q", k, got[k], v)
		}
	}
}

func TestAjustarDNFConf(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", "[main]\nfastestmirror=True\nmax_parallel_downloads=10\n"},
		{"[main]\ngpgcheck=1\nfastestmirror=False\n", "[main]\nfastestmirror=True\nmax_parallel_downloads=10\ngpgcheck=1\n"},
		{"# comentário\n", "[main]\nfastestmirror=True\nmax_parallel_downloads=10\n# comentário\n"},
	}
	for _, tt := range tests {
		if got := ajustarDNFConf(tt.in); got != tt.want {
			t.Errorf("ajustarDNFConf(%q) = %q, esperado %q", tt.in, got, tt.want)
		}
	}
}

func TestLerDesktop(t *testing.T) {
	dir := t.TempDir()
	escrever := func(nome, conteudo string) string {
		p := filepath.Join(dir, nome)
		if err := os.WriteFile(p, []byte(conteudo), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	gnome := []string{"ubuntu", "GNOME"}
	tests := []struct {
		nome, conteudo string
		visivel        bool
		cats           []string
	}{
		{"ok.desktop", "[Desktop Entry]\nType=Application\nCategories=Game;Network;\n[Desktop Action x]\nCategories=Office;\n", true, []string{"Game", "Network"}},
		{"nodisplay.desktop", "[Desktop Entry]\nType=Application\nNoDisplay=true\n", false, nil},
		{"kde.desktop", "[Desktop Entry]\nType=Application\nOnlyShowIn=KDE;\n", false, nil},
		{"notgnome.desktop", "[Desktop Entry]\nType=Application\nNotShowIn=GNOME;\n", false, nil},
		{"ubuntu.desktop", "[Desktop Entry]\nType=Application\nOnlyShowIn=ubuntu;\nCategories=System;\n", true, []string{"System"}},
		{"tryexec.desktop", "[Desktop Entry]\nType=Application\nTryExec=/nao/existe\n", false, nil},
		{"link.desktop", "[Desktop Entry]\nType=Link\n", false, nil},
	}
	for _, tt := range tests {
		cats, vis := lerDesktop(escrever(tt.nome, tt.conteudo), gnome)
		if vis != tt.visivel || !slices.Equal(cats, tt.cats) {
			t.Errorf("%s: visível %v cats %v, esperado %v %v", tt.nome, vis, cats, tt.visivel, tt.cats)
		}
	}

	// ID de subpasta e prioridade: o primeiro diretório vence, mesmo oculto.
	alta, baixa := filepath.Join(dir, "alta"), filepath.Join(dir, "baixa")
	os.MkdirAll(filepath.Join(baixa, "kde"), 0o755)
	os.MkdirAll(alta, 0o755)
	os.WriteFile(filepath.Join(alta, "x.desktop"), []byte("[Desktop Entry]\nType=Application\nHidden=true\n"), 0o644)
	os.WriteFile(filepath.Join(baixa, "x.desktop"), []byte("[Desktop Entry]\nType=Application\n"), 0o644)
	os.WriteFile(filepath.Join(baixa, "kde", "y.desktop"), []byte("[Desktop Entry]\nType=Application\n"), 0o644)
	apps := appsInstalados([]string{alta, baixa}, gnome)
	if _, ok := apps["x.desktop"]; ok {
		t.Error("x.desktop oculto no diretório de maior prioridade não deveria aparecer")
	}
	if _, ok := apps["kde-y.desktop"]; !ok {
		t.Errorf("ID de subpasta não montado: %v", apps)
	}
}

func TestOrganizarPastas(t *testing.T) {
	pastas, err := lerPastas("../../GNOME/pastas.txt")
	if err != nil {
		t.Fatal(err)
	}
	// Amostra real (Ubuntu 26.04 com os Flatpaks do post-install).
	apps := map[string][]string{
		"com.valvesoftware.Steam.desktop":          {"Network", "FileTransfer", "Game"},
		"org.gnome.Boxes.desktop":                  {"GNOME", "GTK", "System", "Development", "Emulator"},
		"com.microsoft.VSCode.desktop":             {"TextEditor", "Development", "IDE"},
		"dev.vencord.Vesktop.desktop":              {"Network"},
		"org.remmina.Remmina.desktop":              {"GTK", "GNOME", "X-GNOME-NetworkSettings", "Network"},
		"org.telegram.desktop.desktop":             {"Chat", "Network", "InstantMessaging", "Qt"},
		"firefox.desktop":                          {"GNOME", "GTK", "Network", "WebBrowser"},
		"org.gnome.Papers.desktop":                 {"GNOME", "GTK", "Office", "Viewer", "Graphics", "2DGraphics", "VectorGraphics"},
		"com.spotify.Client.desktop":               {"Audio", "Music", "Player", "AudioVideo"},
		"org.gnome.DiskUtility.desktop":            {"GNOME", "GTK", "Utility", "X-GNOME-Utilities", "Settings", "HardwareSettings"},
		"org.gnome.Calculator.desktop":             {"GNOME", "GTK", "Utility", "Calculator"},
		"com.github.tchx84.Flatseal.desktop":       {"Utility"},
		"org.libreoffice.LibreOffice.draw.desktop": {"Office", "FlowChart", "Graphics", "2DGraphics", "VectorGraphics"},
		"sem-categoria.desktop":                    nil,
	}
	org := organizarPastas(pastas, apps)
	pastaDe := func(id string) string {
		for i, o := range org {
			if slices.Contains(o.membros, id) {
				return pastas[i].Nome
			}
		}
		return ""
	}
	esperado := map[string]string{
		"com.valvesoftware.Steam.desktop":          "Jogos",
		"org.gnome.Boxes.desktop":                  "Sistema", // forçado em "apps", apesar de Development
		"com.microsoft.VSCode.desktop":             "Desenvolvimento",
		"dev.vencord.Vesktop.desktop":              "Comunicação",
		"org.remmina.Remmina.desktop":              "Internet",
		"org.telegram.desktop.desktop":             "Comunicação",
		"firefox.desktop":                          "Internet",
		"org.gnome.Papers.desktop":                 "Escritório",
		"com.spotify.Client.desktop":               "Multimídia",
		"org.gnome.DiskUtility.desktop":            "Sistema",
		"org.gnome.Calculator.desktop":             "Utilitários",
		"com.github.tchx84.Flatseal.desktop":       "Sistema",
		"org.libreoffice.LibreOffice.draw.desktop": "Escritório",
		"sem-categoria.desktop":                    "",
	}
	for id, want := range esperado {
		if got := pastaDe(id); got != want {
			t.Errorf("%s ficou em %q, esperado %q", id, got, want)
		}
	}
	// O GNOME mostraria a Steam também em Internet e o Boxes em
	// Desenvolvimento (pelas categorias): precisam estar excluídos lá.
	idx := func(nome string) int { return slices.IndexFunc(pastas, func(p pasta) bool { return p.Nome == nome }) }
	if !slices.Contains(org[idx("Internet")].excluidos, "com.valvesoftware.Steam.desktop") ||
		!slices.Contains(org[idx("Desenvolvimento")].excluidos, "org.gnome.Boxes.desktop") {
		t.Errorf("excluded-apps incompletos: Internet %v / Desenvolvimento %v", org[idx("Internet")].excluidos, org[idx("Desenvolvimento")].excluidos)
	}
	// Forçados sem categoria que bata precisam ir em "apps"; os que batem, não.
	if !slices.Contains(org[idx("Comunicação")].apps, "dev.vencord.Vesktop.desktop") ||
		slices.Contains(org[idx("Comunicação")].apps, "org.telegram.desktop.desktop") {
		t.Errorf("apps de Comunicação = %v", org[idx("Comunicação")].apps)
	}

	conf := dconfPastas(pastas, org)
	for _, trecho := range []string{
		"folder-children=['jogos', 'desenvolvimento', 'escritorio', 'comunicacao', 'internet', 'multimidia', 'sistema', 'utilitarios']",
		"[folders/comunicacao]\nname='Comunicação'\ntranslate=false\n",
		"apps=@as []",
	} {
		if !strings.Contains(conf, trecho) {
			t.Errorf("dconf sem %q:\n%s", trecho, conf)
		}
	}
}

func TestEscolherFavoritos(t *testing.T) {
	favs, err := lerFavoritos("../../GNOME/favoritos.txt")
	if err != nil {
		t.Fatal(err)
	}
	// Debian: GNOME Terminal no lugar do Ptyxis, sem loja nem Spotify.
	apps := map[string][]string{
		"org.gnome.Settings.desktop": nil, "org.gnome.Nautilus.desktop": nil,
		"org.gnome.Terminal.desktop": nil, "google-chrome.desktop": nil,
	}
	got := escolherFavoritos(favs, apps)
	want := []string{"org.gnome.Settings.desktop", "org.gnome.Nautilus.desktop", "org.gnome.Terminal.desktop", "google-chrome.desktop"}
	if !slices.Equal(got, want) {
		t.Errorf("escolherFavoritos = %v, esperado %v", got, want)
	}
}
