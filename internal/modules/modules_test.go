package modules

import (
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
