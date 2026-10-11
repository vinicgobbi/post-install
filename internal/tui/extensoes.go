package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---- Tela: extensões do GNOME sem versão compatível ----

// Lista as extensões sem versão para o GNOME Shell da máquina, todas
// desmarcadas: por padrão não são instaladas, e o usuário marca as que quer
// instalar mesmo assim.
func (m Model) updateExtensoes(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	exts := m.cfg.ExtIncompativeis
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	case "up", "k":
		if m.extCursor > 0 {
			m.extCursor--
		}
	case "down", "j":
		if m.extCursor < len(exts)-1 {
			m.extCursor++
		}
	case " ", "x":
		uuid := exts[m.extCursor].UUID
		if i := slices.Index(m.cfg.ForcarExtensoes, uuid); i >= 0 {
			m.cfg.ForcarExtensoes = slices.Delete(m.cfg.ForcarExtensoes, i, i+1)
		} else {
			m.cfg.ForcarExtensoes = append(m.cfg.ForcarExtensoes, uuid)
		}
	case "enter":
		m.state = m.proxima(stateExtensoes)
	}
	return m, nil
}

func (m Model) viewExtensoes() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n")
	b.WriteString(warnStyle.Bold(true).Render(fmt.Sprintf("Extensões sem versão compatível com o GNOME Shell %s", m.cfg.ShellVersion)))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Width(max(20, min(m.w, 100))).Render(
		"Estas extensões não serão instaladas automaticamente. Marque as que quiser instalar mesmo assim — " +
			"elas podem não funcionar corretamente (será instalada a versão mais recente publicada)."))
	b.WriteString("\n\n")

	for i, e := range m.cfg.ExtIncompativeis {
		check := subtleStyle.Render("[ ]")
		if slices.Contains(m.cfg.ForcarExtensoes, e.UUID) {
			check = warnStyle.Render("[✓]")
		}
		nome, pointer := e.UUID, "  "
		if i == m.extCursor {
			pointer, nome = cursorStyle.Render("› "), cursorStyle.Render(nome)
		}
		b.WriteString(pointer + check + " " + nome + "  " + subtleStyle.Render("suporta até o GNOME "+e.MaiorShell) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(help("↑/↓", "mover", "espaço", "instalar mesmo assim", "enter", "continuar", "q", "sair"))
	return b.String()
}
