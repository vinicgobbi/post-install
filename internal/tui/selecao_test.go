package tui

import (
	"testing"

	"github.com/vinicgobbi/post-install/internal/modules"
)

func TestLinhasSelecao(t *testing.T) {
	mods := []modules.Module{{ID: "a", Grupo: "G1"}, {ID: "b", Grupo: "G1"}, {ID: "c", Grupo: "G2"}}
	got := linhasSelecao(mods)
	want := []linhaSelecao{{mod: -1, grupo: "G1"}, {mod: 0}, {mod: 1}, {mod: -1, grupo: "G2"}, {mod: 2}}
	if len(got) != len(want) {
		t.Fatalf("linhasSelecao = %v, esperado %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("linha %d = %v, esperado %v", i, got[i], want[i])
		}
	}
}
