package modules

import (
	"encoding/json"
	"slices"
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
	ativar, desativar, err := lerListaExtensoes("../../GNOME/extensoes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(ativar) != 10 || !slices.Contains(ativar, "gsconnect@andyholmes.github.io") || !slices.Contains(ativar, "ubuntu-dock@ubuntu.com") {
		t.Errorf("ativar = %v", ativar)
	}
	if !slices.Equal(desativar, []string{"ding@rastersoft.com"}) {
		t.Errorf("desativar = %v", desativar)
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
