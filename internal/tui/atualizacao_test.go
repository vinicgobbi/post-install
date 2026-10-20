package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vinicgobbi/post-install/internal/sysinfo"
)

func tecla(m Model, k tea.KeyType) (Model, tea.Cmd) {
	nm, cmd := m.Update(tea.KeyMsg{Type: k})
	return nm.(Model), cmd
}

func TestTelaAtualizacao(t *testing.T) {
	base := Config{Sys: &sysinfo.Info{}, Version: "v1.5.0", User: "ana", ValidateUser: func(string) error { return nil },
		UpdateVersion: "v1.6.0", ApplyUpdate: func() error { return nil }}

	// Ignorar: segue para a tela que viria sem a atualização.
	m := New(base)
	if m.state != stateAtualizacao {
		t.Fatalf("estado inicial = %v, esperado a tela de atualização", m.state)
	}
	m, _ = tecla(m, tea.KeyDown)
	m, _ = tecla(m, tea.KeyEnter)
	if m.state != stateSelect || m.Updated() {
		t.Errorf("depois de ignorar: estado %v, atualizado %v", m.state, m.Updated())
	}

	// Atualizar com sucesso.
	m = New(base)
	m, cmd := tecla(m, tea.KeyEnter)
	if !m.atualizando || cmd == nil {
		t.Fatal("enter em 'Atualizar agora' não iniciou a atualização")
	}
	nm, _ := m.Update(cmd())
	if m = nm.(Model); !m.Updated() {
		t.Error("atualização bem-sucedida não marcou Updated")
	}

	// Falha: mostra o erro e continua permitindo ignorar.
	falha := base
	falha.ApplyUpdate = func() error { return errors.New("sem rede") }
	m = New(falha)
	m, cmd = tecla(m, tea.KeyEnter)
	nm, _ = m.Update(cmd())
	if m = nm.(Model); m.Updated() || m.updErr != "sem rede" {
		t.Errorf("falha: atualizado %v, erro %q", m.Updated(), m.updErr)
	}

	// Dry-run: sem tela, só o aviso no topo.
	dry := base
	dry.DryRun = true
	if m = New(dry); m.state == stateAtualizacao {
		t.Error("dry-run não deveria abrir a tela de atualização")
	}
}
