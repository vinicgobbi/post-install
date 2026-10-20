package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---- Tela: nova versão disponível ----

// Primeira tela quando há uma release mais nova: atualizar agora (baixa,
// confere e substitui o pacote; depois é preciso executar de novo) ou
// ignorar e seguir com a versão atual.

type atualizadoMsg struct{ err error }

func (m Model) updateAtualizacao(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.atualizando {
		// Interromper no meio deixaria o pacote pela metade.
		return m, nil
	}
	if m.atualizado {
		switch msg.String() {
		case "ctrl+c", "q", "esc", "enter":
			return m, tea.Quit
		}
		return m, nil
	}
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	case "up", "k":
		m.updChoice = 0
	case "down", "j":
		m.updChoice = 1
	case "enter", " ":
		if m.updChoice == 1 {
			m.state = m.depoisAtualizacao
			return m, nil
		}
		m.atualizando, m.updErr = true, ""
		apply := m.cfg.ApplyUpdate
		return m, func() tea.Msg { return atualizadoMsg{err: apply()} }
	}
	return m, nil
}

func (m Model) viewAtualizacao() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n")
	largura := lipgloss.NewStyle().Width(max(20, min(m.w, 100)))

	if m.atualizado {
		b.WriteString(okStyle.Bold(true).Render(fmt.Sprintf("✓ post-install atualizado para %s", m.cfg.UpdateVersion)))
		b.WriteString("\n\n")
		b.WriteString(largura.Render("Esta execução ainda é a versão antiga. Execute de novo para usar a nova:"))
		b.WriteString("\n\n  ")
		b.WriteString(accentStyle.Render(m.cfg.RerunCommand))
		b.WriteString("\n\n")
		b.WriteString(help("enter", "sair"))
		return b.String()
	}

	b.WriteString(infoStyle.Bold(true).Render(fmt.Sprintf("Nova versão disponível: %s", m.cfg.UpdateVersion)))
	b.WriteString(subtleStyle.Render(fmt.Sprintf("  (você está na %s)", m.cfg.Version)))
	b.WriteString("\n")
	b.WriteString(largura.Render(subtleStyle.Render(
		"Atualizar baixa a release, confere o SHA-256 e substitui o binário (e, no pacote da release, " +
			"GNOME/, Wallpapers/ e os READMEs — os .ovpn e os logs ficam intactos). Depois é preciso executar de novo.")))
	b.WriteString("\n\n")

	if m.atualizando {
		b.WriteString(m.spin.View() + " Baixando e conferindo " + m.cfg.UpdateVersion + "…\n\n")
		b.WriteString(subtleStyle.Render("Aguarde: interromper agora deixaria o pacote pela metade."))
		return b.String()
	}

	opcoes := []string{"Atualizar agora", "Ignorar e continuar com a " + m.cfg.Version}
	for i, o := range opcoes {
		pointer, radio, nome := "  ", subtleStyle.Render("( )"), o
		if i == m.updChoice {
			pointer, radio, nome = cursorStyle.Render("› "), cursorStyle.Render("(•)"), cursorStyle.Render(o)
		}
		b.WriteString(pointer + radio + " " + nome + "\n")
	}
	if m.updErr != "" {
		b.WriteString("\n" + largura.Render(errStyle.Render("✗ Falha ao atualizar: "+m.updErr)) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(help("↑/↓", "mover", "enter", "escolher", "q", "sair"))
	return b.String()
}
