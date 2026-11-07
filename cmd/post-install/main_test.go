package main

import (
	"slices"
	"testing"

	"github.com/vinicgobbi/post-install/internal/modules"
)

func TestInitialSelection(t *testing.T) {
	all := modules.All()
	// Uma distribuição onde só "go" e "mise" existem.
	var parcial []modules.Module
	for _, m := range all {
		if m.ID == "go" || m.ID == "mise" {
			parcial = append(parcial, m)
		}
	}

	sel, avisos, err := initialSelection(all, "", "")
	if err != nil || len(avisos) != 0 || slices.Contains(sel, false) {
		t.Errorf("sem flags: todos marcados, sem avisos (sel %v, avisos %v, err %v)", sel, avisos, err)
	}

	sel, avisos, err = initialSelection(parcial, "go,podman", "")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(sel, []bool{false, true}) && !slices.Equal(sel, []bool{true, false}) {
		t.Errorf("só o go deveria ficar marcado: %v", sel)
	}
	if len(avisos) != 1 {
		t.Errorf("podman indisponível deveria gerar um aviso: %v", avisos)
	}

	if _, _, err := initialSelection(parcial, "", "nao_existe"); err == nil {
		t.Error("um ID que não existe em nenhuma distribuição deveria ser erro")
	}
}
