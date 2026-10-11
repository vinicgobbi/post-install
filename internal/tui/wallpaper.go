package tui

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ---- Tela: papel de parede ----

// A última opção da lista é "não alterar".
func (m Model) updateWallpaper(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(m.cfg.Wallpapers) + 1
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	case "up", "k":
		if m.wallCursor > 0 {
			m.wallCursor--
		}
	case "down", "j":
		if m.wallCursor < n-1 {
			m.wallCursor++
		}
	case "enter", " ":
		m.status = ""
		if m.wallCursor < len(m.cfg.Wallpapers) {
			m.cfg.Wallpaper = m.cfg.Wallpapers[m.wallCursor]
		} else {
			// Sem imagem, o módulo não teria o que fazer: desmarca.
			m.cfg.Wallpaper = ""
			for i, mod := range m.cfg.Modules {
				if mod.ID == "papel_parede" {
					m.sel[i] = false
				}
			}
		}
		m.state = m.proxima(stateWallpaper)
	}
	return m, nil
}

func (m Model) viewWallpaper() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n")
	b.WriteString(boldStyle.Render("Qual papel de parede deseja aplicar?"))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render("A imagem é copiada para ~/.local/share/backgrounds e aplicada ao final da configuração."))
	b.WriteString("\n\n")

	for i, w := range m.cfg.Wallpapers {
		b.WriteString(m.wallLine(i, filepath.Base(w), imageInfo(w)))
	}
	b.WriteString(m.wallLine(len(m.cfg.Wallpapers), "Não alterar o papel de parede", ""))
	b.WriteString("\n")
	b.WriteString(help("↑/↓", "mover", "enter", "escolher", "q", "sair"))
	return b.String()
}

func (m Model) wallLine(i int, name, info string) string {
	pointer, radio := "  ", subtleStyle.Render("( )")
	if i == m.wallCursor {
		pointer, radio, name = cursorStyle.Render("› "), cursorStyle.Render("(•)"), cursorStyle.Render(name)
	}
	if info != "" {
		info = "  " + subtleStyle.Render(info)
	}
	return pointer + radio + " " + name + info + "\n"
}

// imageInfo descreve a imagem como "2000×1158 · 3,3 MB".
func imageInfo(path string) string {
	var parts []string
	if f, err := os.Open(path); err == nil {
		if cfg, _, err := image.DecodeConfig(f); err == nil {
			parts = append(parts, fmt.Sprintf("%d×%d", cfg.Width, cfg.Height))
		}
		f.Close()
	}
	if st, err := os.Stat(path); err == nil {
		parts = append(parts, humanSize(st.Size()))
	}
	return strings.Join(parts, " · ")
}

// humanSize formata em KB abaixo de 1 MB (senão "0,0 MB") e em MB acima.
func humanSize(n int64) string {
	if n < 1e6 {
		return fmt.Sprintf("%d KB", (n+500)/1000)
	}
	return strings.Replace(fmt.Sprintf("%.1f MB", float64(n)/1e6), ".", ",", 1)
}
