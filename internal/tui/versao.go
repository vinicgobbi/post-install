package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---- Tela: versão da distribuição não testada ----

// Aparece quando a versão da distribuição é mais nova que todas as testadas
// (sysinfo.Info.NaoTestada), fora do dry-run e sem --allow-untested-version.
// Vem depois da oferta de atualização: uma release nova pode já suportar a
// versão. A opção padrão é sair.

func (m Model) updateVersao(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	case "up", "k":
		m.versaoChoice = 0
	case "down", "j":
		m.versaoChoice = 1
	case "enter", " ":
		if m.versaoChoice == 0 {
			return m, tea.Quit
		}
		m.state = m.depoisVersao
	}
	return m, nil
}

func (m Model) viewVersao() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n")
	largura := lipgloss.NewStyle().Width(max(20, min(m.w, 100)))

	b.WriteString(warnStyle.Bold(true).Render("Versão não testada: " + m.cfg.Sys.PrettyName))
	b.WriteString("\n")
	b.WriteString(largura.Render(subtleStyle.Render(
		"Esta versão do post-install foi testada só com " + strings.Join(m.cfg.Sys.Testadas, ", ") + ". " +
			"Continuar pode falhar ou deixar o sistema num estado incerto. Se houver uma versão mais nova do " +
			"post-install, ela pode já suportar este sistema.")))
	b.WriteString("\n\n")

	opcoes := []string{"Sair", "Continuar, aceitando os riscos"}
	for i, o := range opcoes {
		pointer, radio, nome := "  ", subtleStyle.Render("( )"), o
		if i == m.versaoChoice {
			pointer, radio, nome = cursorStyle.Render("› "), cursorStyle.Render("(•)"), cursorStyle.Render(o)
		}
		b.WriteString(pointer + radio + " " + nome + "\n")
	}
	b.WriteString("\n")
	b.WriteString(help("↑/↓", "mover", "enter", "escolher", "q", "sair"))
	return b.String()
}
