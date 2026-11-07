package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vinicgobbi/post-install/internal/sysinfo"
)

func TestTelaVersao(t *testing.T) {
	sys := &sysinfo.Info{PrettyName: "Ubuntu 26.10", NaoTestada: true, Testadas: []string{"22.04", "24.04", "26.04"}}
	base := Config{Sys: sys, Version: "v3.1.0", User: "ana", ValidateUser: func(string) error { return nil }}

	// O padrão é sair.
	m := New(base)
	if m.state != stateVersao {
		t.Fatalf("estado inicial = %v, esperado a tela de versão", m.state)
	}
	if m.viewVersao() == "" {
		t.Error("tela vazia")
	}
	if _, cmd := tecla(m, tea.KeyEnter); cmd == nil {
		t.Error("enter em 'Sair' não saiu")
	}

	// Continuar segue para a tela que viria sem a pergunta.
	m, _ = tecla(m, tea.KeyDown)
	m, _ = tecla(m, tea.KeyEnter)
	if m.state != stateSelect {
		t.Errorf("depois de continuar: estado %v, esperado o checklist", m.state)
	}

	// Sem pergunta: no dry-run, com --allow-untested-version e em versão testada.
	for nome, cfg := range map[string]Config{
		"dry-run": func() Config { c := base; c.DryRun = true; return c }(),
		"aceita":  func() Config { c := base; c.VersaoAceita = true; return c }(),
		"testada": func() Config { c := base; c.Sys = &sysinfo.Info{}; return c }(),
	} {
		if m := New(cfg); m.state == stateVersao {
			t.Errorf("%s: não deveria perguntar", nome)
		}
	}

	// Com versão nova do post-install, a oferta de atualização vem antes, e
	// ignorá-la leva à pergunta da versão.
	cfg := base
	cfg.UpdateVersion, cfg.ApplyUpdate = "v3.2.0", func() error { return nil }
	m = New(cfg)
	if m.state != stateAtualizacao {
		t.Fatalf("estado inicial = %v, esperado a tela de atualização", m.state)
	}
	m, _ = tecla(m, tea.KeyDown)
	m, _ = tecla(m, tea.KeyEnter)
	if m.state != stateVersao {
		t.Errorf("depois de ignorar a atualização: estado %v, esperado a tela de versão", m.state)
	}
}
