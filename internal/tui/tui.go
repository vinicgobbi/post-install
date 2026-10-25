// Package tui é a interface interativa (Bubble Tea): usuário alvo → escolha
// dos módulos → confirmação → execução com progresso ao vivo → resumo.
package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/vinicgobbi/post-install/internal/engine"
	"github.com/vinicgobbi/post-install/internal/modules"
	"github.com/vinicgobbi/post-install/internal/runlog"
	"github.com/vinicgobbi/post-install/internal/sysinfo"
)

type Config struct {
	Sys      *sysinfo.Info
	Version  string
	DryRun   bool
	Modules  []modules.Module
	Selected []bool // seleção inicial
	// User já validado (pula a tela de usuário quando preenchido).
	User string
	// DetectedUser é quem executou o sudo (sugerido na tela de usuário).
	DetectedUser string
	ValidateUser func(string) error
	// Wallpapers são as imagens disponíveis; Wallpaper é a escolhida (vazio
	// = não alterar). Com WallChosen (veio de --wallpaper), não pergunta.
	Wallpapers []string
	Wallpaper  string
	WallChosen bool
	// ExtIncompativeis são as extensões do GNOME sem versão para o
	// GNOME Shell da máquina (ShellVersion); ForcarExtensoes, as que o
	// usuário quer instalar mesmo assim. Com ExtChosen (veio de
	// --force-extensions), não pergunta.
	ShellVersion     string
	ExtIncompativeis []modules.ExtInfo
	ForcarExtensoes  []string
	ExtChosen        bool
	// Start roda a execução dos módulos escolhidos, enviando eventos do
	// engine (e entradas de log) para emit. Bloqueia até terminar.
	Start    func(ctx context.Context, esc modules.Escolhas, mods []modules.Module, emit func(any))
	LogPaths []string
	// UpdateVersion é a release mais nova disponível (vazio = nenhuma).
	// Fora do dry-run, a primeira tela oferece rodar ApplyUpdate; depois de
	// atualizar, mostra RerunCommand e sai. No dry-run, só avisa no topo.
	UpdateVersion string
	ApplyUpdate   func() error
	RerunCommand  string
}

type state int

const (
	stateUser state = iota
	stateAtualizacao
	stateWallpaper
	stateExtensoes
	stateSelect
	stateConfirm
	stateRunning
	stateDone
)

const maxLogLines = 3000

type Model struct {
	cfg   Config
	state state
	w, h  int

	input      textinput.Model
	userErr    string
	userChoice int  // 0 = usuário detectado, 1 = outro
	typing     bool // campo de texto visível

	wallCursor int
	extCursor  int

	updChoice         int // 0 = atualizar, 1 = ignorar
	atualizando       bool
	atualizado        bool
	updErr            string
	depoisAtualizacao state

	cursor int
	sel    []bool
	status string

	// Execução.
	runMods   []modules.Module
	results   []engine.Result
	current   int
	started   time.Time
	elapsed   time.Duration
	events    chan any
	cancel    context.CancelFunc
	cancelAsk bool
	canceling bool
	logs      []runlog.Entry
	vp        viewport.Model
	follow    bool
	spin      spinner.Model
	bar       progress.Model
	finished  *engine.FinishedEvent
}

// Result devolve o resultado final (nil se o usuário saiu antes de executar).
func (m Model) Result() *engine.FinishedEvent { return m.finished }

// Updated diz se o post-install foi atualizado nesta sessão (é preciso
// executar de novo).
func (m Model) Updated() bool { return m.atualizado }

func New(cfg Config) Model {
	in := textinput.New()
	in.Placeholder = "nome do usuário"
	in.Prompt = "› "
	in.CharLimit = 64

	sel := make([]bool, len(cfg.Modules))
	copy(sel, cfg.Selected)

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = accentStyle

	bar := progress.New(progress.WithGradient("#7C3AED", "#22C55E"), progress.WithoutPercentage())

	m := Model{
		cfg:    cfg,
		input:  in,
		sel:    sel,
		spin:   sp,
		bar:    bar,
		follow: true,
		vp:     viewport.New(80, 10),
	}
	if cfg.User != "" {
		m.state = m.afterUser()
	}
	if cfg.DetectedUser == "" {
		m.typing = true
		m.input.Focus()
	}
	if cfg.UpdateVersion != "" && !cfg.DryRun && cfg.ApplyUpdate != nil {
		m.depoisAtualizacao, m.state = m.state, stateAtualizacao
	}
	return m
}

// afterUser decide a tela seguinte à do usuário.
func (m *Model) afterUser() state { return m.proxima(stateUser) }

// proxima devolve a próxima pergunta depois de atual, pulando as que não se
// aplicam, até chegar ao checklist:
//   - papel de parede: há imagens, o módulo está marcado e não veio --wallpaper;
//   - extensões incompatíveis: há alguma, o módulo está marcado e não veio
//     --force-extensions.
func (m *Model) proxima(atual state) state {
	for _, s := range []state{stateWallpaper, stateExtensoes} {
		if s <= atual {
			continue
		}
		switch {
		case s == stateWallpaper && !m.cfg.WallChosen && len(m.cfg.Wallpapers) > 0 && m.marcado("papel_parede"):
			return s
		case s == stateExtensoes && !m.cfg.ExtChosen && len(m.cfg.ExtIncompativeis) > 0 && m.marcado("extensoes_gnome"):
			return s
		}
	}
	return stateSelect
}

func (m *Model) marcado(id string) bool {
	for i, mod := range m.cfg.Modules {
		if mod.ID == id {
			return m.sel[i]
		}
	}
	return false
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spin.Tick)
}

type eventsMsg []any
type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// listen espera o próximo evento do engine e já drena os que estiverem na
// fila, para não redesenhar a tela a cada linha de saída do apt/dnf.
func listen(ch chan any) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		batch := []any{ev}
		for len(batch) < 500 {
			select {
			case ev, ok := <-ch:
				if !ok {
					return eventsMsg(batch)
				}
				batch = append(batch, ev)
			default:
				return eventsMsg(batch)
			}
		}
		return eventsMsg(batch)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Descontando o padding aplicado em View.
		m.w, m.h = msg.Width-4, msg.Height-1
		m.resize()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		pm, cmd := m.bar.Update(msg)
		m.bar = pm.(progress.Model)
		return m, cmd

	case tickMsg:
		if m.state == stateRunning {
			m.elapsed = time.Since(m.started)
			return m, tick()
		}
		return m, nil

	case eventsMsg:
		return m.handleEvents(msg)

	case atualizadoMsg:
		m.atualizando = false
		if msg.err != nil {
			m.updErr = msg.err.Error()
		} else {
			m.atualizado = true
		}
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case stateUser:
			return m.updateUser(msg)
		case stateAtualizacao:
			return m.updateAtualizacao(msg)
		case stateWallpaper:
			return m.updateWallpaper(msg)
		case stateExtensoes:
			return m.updateExtensoes(msg)
		case stateSelect:
			return m.updateSelect(msg)
		case stateConfirm:
			return m.updateConfirm(msg)
		case stateRunning:
			return m.updateRunning(msg)
		case stateDone:
			return m.updateDone(msg)
		}
	}

	if m.state == stateUser {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

// ---- Tela: usuário ----

// Com um usuário detectado (quem chamou o sudo), a tela oferece ele ou
// "outro usuário"; só em "outro" (ou sem detecção) aparece o campo de texto.

func (m Model) updateUser(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !m.typing {
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			return m, tea.Quit
		case "up", "k":
			m.userChoice = 0
		case "down", "j":
			m.userChoice = 1
		case "enter", " ":
			if m.userChoice == 1 {
				m.typing = true
				m.input.Focus()
				return m, textinput.Blink
			}
			return m.chooseUser(m.cfg.DetectedUser)
		}
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.cfg.DetectedUser == "" {
			return m, tea.Quit
		}
		m.typing, m.userErr = false, ""
		m.input.Blur()
		return m, nil
	case "enter":
		nome := strings.TrimSpace(m.input.Value())
		if nome == "" {
			m.userErr = "Digite um nome de usuário."
			return m, nil
		}
		return m.chooseUser(nome)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.userErr = ""
	return m, cmd
}

func (m Model) chooseUser(nome string) (tea.Model, tea.Cmd) {
	if err := m.cfg.ValidateUser(nome); err != nil {
		m.userErr = err.Error()
		return m, nil
	}
	m.cfg.User = nome
	m.userErr = ""
	m.state = m.afterUser()
	return m, nil
}

func (m Model) viewUser() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n")
	b.WriteString(boldStyle.Render("Para qual usuário do sistema devo configurar o ambiente?"))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render("Grupos, shell, dotfiles, Node, Rust e Claude Code serão aplicados a este usuário."))
	b.WriteString("\n\n")

	if d := m.cfg.DetectedUser; d != "" {
		opcoes := []struct{ nome, info string }{
			{d, "usuário detectado (quem executou o sudo)"},
			{"Outro usuário…", ""},
		}
		for i, o := range opcoes {
			pointer, radio, nome := "  ", subtleStyle.Render("( )"), o.nome
			if i == m.userChoice {
				pointer, radio, nome = cursorStyle.Render("› "), cursorStyle.Render("(•)"), cursorStyle.Render(o.nome)
			}
			if o.info != "" {
				nome += "  " + subtleStyle.Render(o.info)
			}
			b.WriteString(pointer + radio + " " + nome + "\n")
		}
		b.WriteString("\n")
	}

	if m.typing {
		b.WriteString(m.input.View())
		b.WriteString("\n")
	}
	if m.userErr != "" {
		b.WriteString(errStyle.Render("✗ " + m.userErr))
	}
	b.WriteString("\n\n")
	switch {
	case !m.typing:
		b.WriteString(help("↑/↓", "mover", "enter", "escolher", "esc", "sair"))
	case m.cfg.DetectedUser != "":
		b.WriteString(help("enter", "continuar", "esc", "voltar"))
	default:
		b.WriteString(help("enter", "continuar", "esc", "sair"))
	}
	return b.String()
}

// ---- Tela: seleção de módulos ----

func (m *Model) countSelected() int {
	n := 0
	for _, s := range m.sel {
		if s {
			n++
		}
	}
	return n
}

func (m Model) updateSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	mods := m.cfg.Modules
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(mods)-1 {
			m.cursor++
		}
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		m.cursor = len(mods) - 1
	case " ", "x":
		m.status = ""
		if m.sel[m.cursor] {
			if removed := engine.DeselectWithDependents(mods, m.sel, m.cursor); len(removed) > 0 {
				m.status = warnStyle.Render("Também desmarcado (depende deste): " + strings.Join(removed, ", "))
			}
		} else if added := engine.SelectWithDeps(mods, m.sel, m.cursor); len(added) > 0 {
			m.status = infoStyle.Render("Também marcado (dependência): " + strings.Join(added, ", "))
		}
	case "a":
		all := m.countSelected() == len(mods)
		for i := range m.sel {
			m.sel[i] = !all
		}
		m.status = ""
	case "enter":
		if m.countSelected() == 0 {
			m.status = warnStyle.Render("Nenhum módulo selecionado.")
			return m, nil
		}
		m.state = stateConfirm
	}
	return m, nil
}

func (m Model) viewSelect() string {
	mods := m.cfg.Modules
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n")
	fmt.Fprintf(&b, "%s %s\n\n", boldStyle.Render("Escolha o que deseja executar"),
		subtleStyle.Render(fmt.Sprintf("(%d de %d selecionados)", m.countSelected(), len(mods))))

	detail := m.moduleDetail(mods[m.cursor])
	fixed := lipgloss.Height(m.header()) + 3 + lipgloss.Height(detail) + 3
	rows := max(3, m.h-fixed)
	start, end := window(m.cursor, len(mods), rows)

	for i := start; i < end; i++ {
		check := subtleStyle.Render("[ ]")
		if m.sel[i] {
			check = okStyle.Render("[✓]")
		}
		title := mods[i].Title
		pointer := "  "
		if i == m.cursor {
			pointer = cursorStyle.Render("› ")
			title = cursorStyle.Render(title)
		}
		b.WriteString(pointer + check + " " + title + "\n")
	}
	if end < len(mods) {
		b.WriteString(subtleStyle.Render(fmt.Sprintf("   ↓ mais %d", len(mods)-end)) + "\n")
	} else {
		b.WriteString("\n")
	}

	b.WriteString(detail + "\n")
	b.WriteString(lipgloss.NewStyle().Width(max(20, m.w)).Render(m.status) + "\n")
	b.WriteString(help("↑/↓", "mover", "espaço", "marcar", "a", "todos", "enter", "continuar", "q", "sair"))
	return b.String()
}

func (m Model) moduleDetail(mod modules.Module) string {
	width := max(30, min(m.w, 100)-4)
	var lines []string
	lines = append(lines, accentStyle.Render(mod.Title))
	lines = append(lines, lipgloss.NewStyle().Width(width).Render(mod.Desc))

	var reqs, usedBy []string
	for _, d := range mod.Deps {
		for _, o := range m.cfg.Modules {
			if o.ID == d {
				reqs = append(reqs, o.Title)
			}
		}
	}
	for _, o := range m.cfg.Modules {
		for _, d := range o.Deps {
			if d == mod.ID {
				usedBy = append(usedBy, o.Title)
			}
		}
	}
	if len(reqs) > 0 {
		lines = append(lines, subtleStyle.Render("Requer: "+strings.Join(reqs, ", ")))
	}
	if len(usedBy) > 0 {
		lines = append(lines, subtleStyle.Render("Necessário para: "+strings.Join(usedBy, ", ")))
	}
	return sectionStyle.Width(width + 2).Render(strings.Join(lines, "\n"))
}

// window devolve o intervalo [start, end) de itens visíveis para manter o
// cursor na tela.
func window(cursor, total, rows int) (int, int) {
	if total <= rows {
		return 0, total
	}
	start := max(0, cursor-rows/2)
	end := start + rows
	if end > total {
		end = total
		start = end - rows
	}
	return start, end
}

// ---- Tela: confirmação ----

func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "backspace", "n":
		m.state = stateSelect
	case "enter", "s", "y":
		return m.startRun()
	}
	return m, nil
}

func (m Model) viewConfirm() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n")
	n := m.countSelected()
	fmt.Fprintf(&b, "%s\n\n", boldStyle.Render(fmt.Sprintf("Iniciar a configuração com os %d módulos abaixo?", n)))

	var items []string
	for i, mod := range m.cfg.Modules {
		if m.sel[i] {
			items = append(items, fmt.Sprintf("%s %s", subtleStyle.Render(fmt.Sprintf("%2d.", len(items)+1)), mod.Title))
		}
	}
	// Em duas colunas quando não cabe na altura da tela.
	rows := max(5, m.h-lipgloss.Height(m.header())-8)
	if len(items) > rows && m.w >= 90 {
		half := (len(items) + 1) / 2
		left := strings.Join(items[:half], "\n")
		right := strings.Join(items[half:], "\n")
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(45).Render(left), right))
	} else {
		b.WriteString(strings.Join(items, "\n"))
	}
	b.WriteString("\n\n")
	for i, mod := range m.cfg.Modules {
		if mod.ID == "papel_parede" && m.sel[i] {
			nome := "não alterar"
			if m.cfg.Wallpaper != "" {
				nome = filepath.Base(m.cfg.Wallpaper)
			}
			b.WriteString(subtleStyle.Render("Papel de parede: ") + accentStyle.Render(nome) + "\n\n")
		}
	}
	if m.marcado("extensoes_gnome") && len(m.cfg.ForcarExtensoes) > 0 {
		b.WriteString(warnStyle.Render("Instalar mesmo sem versão compatível (podem não funcionar): "+strings.Join(m.cfg.ForcarExtensoes, ", ")) + "\n\n")
	}
	if m.cfg.DryRun {
		b.WriteString(warnStyle.Render("Modo dry-run: nada será alterado no sistema, os comandos só serão listados.") + "\n\n")
	}
	b.WriteString(help("enter", "iniciar", "esc", "voltar", "q", "sair"))
	return b.String()
}

// ---- Tela: execução ----

func (m Model) startRun() (tea.Model, tea.Cmd) {
	m.runMods = nil
	for i, mod := range m.cfg.Modules {
		if m.sel[i] {
			m.runMods = append(m.runMods, mod)
		}
	}
	m.results = make([]engine.Result, len(m.runMods))
	for i, mod := range m.runMods {
		m.results[i] = engine.Result{Module: mod, Status: engine.Pending}
	}
	m.current = -1
	m.state = stateRunning
	m.started = time.Now()
	m.events = make(chan any, 2048)

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	ch, mods := m.events, m.runMods
	esc := modules.Escolhas{User: m.cfg.User, Wallpaper: m.cfg.Wallpaper, ForcarExtensoes: m.cfg.ForcarExtensoes}
	go func() {
		m.cfg.Start(ctx, esc, mods, func(ev any) { ch <- ev })
		close(ch)
	}()
	m.resize()
	return m, tea.Batch(listen(m.events), tick(), m.spin.Tick)
}

func (m Model) handleEvents(batch eventsMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	logChanged := false
	for _, ev := range batch {
		switch ev := ev.(type) {
		case runlog.Entry:
			m.logs = append(m.logs, ev)
			logChanged = true
		case engine.ModuleStartEvent:
			m.current = ev.Index
			m.results[ev.Index].Status = engine.Running
		case engine.ModuleDoneEvent:
			m.results[ev.Index] = ev.Result
			cmds = append(cmds, m.bar.SetPercent(m.progress()))
		case engine.FinishedEvent:
			m.finished = &ev
			m.elapsed = ev.Elapsed
			m.state = stateDone
		}
	}
	if len(m.logs) > maxLogLines {
		m.logs = m.logs[len(m.logs)-maxLogLines:]
	}
	if logChanged {
		m.refreshLog()
	}
	if m.state == stateRunning {
		cmds = append(cmds, listen(m.events))
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) progress() float64 {
	if len(m.results) == 0 {
		return 0
	}
	done := 0
	for _, r := range m.results {
		if r.Status != engine.Pending && r.Status != engine.Running {
			done++
		}
	}
	return float64(done) / float64(len(m.results))
}

func (m Model) updateRunning(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		if m.canceling {
			return m, nil
		}
		if !m.cancelAsk {
			m.cancelAsk = true
			return m, nil
		}
		m.canceling = true
		m.cancel()
		return m, nil
	case "esc", "n":
		m.cancelAsk = false
		return m, nil
	case "end", "G":
		m.follow = true
		m.vp.GotoBottom()
		return m, nil
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	m.follow = m.vp.AtBottom()
	return m, cmd
}

func (m *Model) listWidth() int {
	return min(40, max(28, m.w/3))
}

func (m *Model) stacked() bool { return m.w < 90 }

func (m *Model) resize() {
	m.bar.Width = max(20, min(m.w-24, 80))
	if m.w == 0 {
		return
	}
	headerH := lipgloss.Height(m.header())
	if m.stacked() {
		m.vp.Width = m.w - 4
		m.vp.Height = max(5, m.h-headerH-13)
	} else {
		m.vp.Width = m.w - m.listWidth() - 4 - 4
		m.vp.Height = max(5, m.h-headerH-5)
	}
	m.refreshLog()
}

func (m *Model) refreshLog() {
	var b strings.Builder
	for i, e := range m.logs {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(renderEntry(e, m.vp.Width))
	}
	m.vp.SetContent(b.String())
	if m.follow {
		m.vp.GotoBottom()
	}
}

func renderEntry(e runlog.Entry, width int) string {
	var line string
	switch e.Level {
	case runlog.Info:
		line = infoStyle.Render("• " + e.Text)
	case runlog.Success:
		line = okStyle.Render("✓ " + e.Text)
	case runlog.Warn:
		line = warnStyle.Render("! " + e.Text)
	case runlog.Error:
		line = errStyle.Render("✗ " + e.Text)
	case runlog.Step:
		line = accentStyle.Render("▶ " + e.Text)
	default:
		line = subtleStyle.Render("  " + e.Text)
	}
	if width > 0 {
		line = ansi.Truncate(line, width, "…")
	}
	return line
}

func statusIcon(s engine.Status, spin string) string {
	switch s {
	case engine.Running:
		return spin
	case engine.Done:
		return okStyle.Render("✓")
	case engine.DoneWithWarnings:
		return warnStyle.Render("!")
	case engine.Failed:
		return errStyle.Render("✗")
	case engine.Skipped:
		return warnStyle.Render("⤼")
	case engine.Canceled:
		return subtleStyle.Render("⊘")
	default:
		return subtleStyle.Render("·")
	}
}

func (m Model) viewRunning() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n")

	done := int(m.progress()*float64(len(m.results)) + 0.5)
	fmt.Fprintf(&b, "%s  %s  %s\n\n", m.bar.View(),
		boldStyle.Render(fmt.Sprintf("%d/%d", done, len(m.results))),
		subtleStyle.Render(fmtDuration(m.elapsed)))

	listW := m.listWidth()
	listH := m.vp.Height
	if m.stacked() {
		listW, listH = m.w-4, 6
	}
	start, end := window(max(0, m.current), len(m.results), listH)
	var list []string
	for i := start; i < end; i++ {
		r := m.results[i]
		title := r.Module.Title
		if r.Status == engine.Running {
			title = boldStyle.Render(title)
		} else if r.Status == engine.Pending {
			title = subtleStyle.Render(title)
		}
		list = append(list, ansi.Truncate(statusIcon(r.Status, m.spin.View())+" "+title, listW, "…"))
	}
	listBox := sectionStyle.Width(listW + 2).Height(listH).Render(strings.Join(list, "\n"))
	logBox := sectionStyle.Width(m.vp.Width + 2).Render(m.vp.View())

	if m.stacked() {
		b.WriteString(lipgloss.JoinVertical(lipgloss.Left, listBox, logBox))
	} else {
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, listBox, logBox))
	}
	b.WriteString("\n")

	switch {
	case m.canceling:
		b.WriteString(warnStyle.Render("Cancelando... aguardando o comando atual encerrar."))
	case m.cancelAsk:
		b.WriteString(warnStyle.Render("Cancelar a execução? ") + help("ctrl+c", "sim, cancelar", "esc", "continuar"))
	default:
		b.WriteString(help("↑/↓ pgup/pgdn", "rolar saída", "G", "seguir saída", "ctrl+c", "cancelar"))
	}
	return b.String()
}

// ---- Tela: resumo ----

func (m Model) updateDone(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "enter", "esc":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) viewDone() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n")
	b.WriteString(Summary(m.finished.Results, m.finished.Elapsed, m.cfg.LogPaths))
	b.WriteString("\n\n")
	b.WriteString(help("enter/q", "sair"))
	return b.String()
}

// Summary é o resumo final, usado pela TUI e impresso de novo no terminal
// depois que ela fecha.
func Summary(results []engine.Result, elapsed time.Duration, logPaths []string) string {
	var b strings.Builder
	counts := map[engine.Status]int{}
	nameW := 0
	for _, r := range results {
		nameW = max(nameW, lipgloss.Width(r.Module.Title))
	}
	for _, r := range results {
		counts[r.Status]++
		line := fmt.Sprintf("%s %-*s", statusIcon(r.Status, "…"), nameW, r.Module.Title)
		if r.Duration > 0 {
			line += subtleStyle.Render("  " + fmtDuration(r.Duration))
		}
		switch {
		case r.Status == engine.DoneWithWarnings:
			line += warnStyle.Render(fmt.Sprintf("  %d aviso(s)", r.Warnings))
		case r.Err != nil && r.Status != engine.Canceled:
			line += errStyle.Render("  " + r.Err.Error())
		}
		b.WriteString(line + "\n")
	}

	ok := counts[engine.Done] + counts[engine.DoneWithWarnings]
	b.WriteString("\n")
	title := okStyle.Bold(true).Render("Concluído")
	switch {
	case counts[engine.Canceled] > 0:
		title = errStyle.Bold(true).Render("Cancelado")
	case counts[engine.Failed] > 0:
		title = errStyle.Bold(true).Render("Concluído com falhas")
	}
	fmt.Fprintf(&b, "%s  %s\n", title, subtleStyle.Render(fmt.Sprintf(
		"%d ok · %d falha(s) · %d pulado(s) · %d cancelado(s) · tempo total %s",
		ok, counts[engine.Failed], counts[engine.Skipped], counts[engine.Canceled], fmtDuration(elapsed))))
	if ok > 0 {
		b.WriteString(infoStyle.Render("Recomenda-se reiniciar a máquina para aplicar as mudanças de grupo e kernel.") + "\n")
	}
	for _, p := range logPaths {
		if p != "" {
			b.WriteString(subtleStyle.Render("Log: "+p) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func fmtDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d >= time.Hour {
		return fmt.Sprintf("%dh%02dm%02ds", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
}

// ---- Comum ----

func (m Model) header() string {
	title := titleStyle.Render("POST-INSTALL")
	if m.cfg.Version != "" {
		title += " " + subtleStyle.Render(m.cfg.Version)
	}
	if m.cfg.DryRun {
		title += " " + warnStyle.Bold(true).Render("[dry-run]")
	}
	info := subtleStyle.Render(m.cfg.Sys.Summary() + " · " + m.cfg.Sys.DesktopName())
	if m.cfg.User != "" {
		info += subtleStyle.Render(" · usuário ") + accentStyle.Render(m.cfg.User)
	}
	out := title + "\n" + info
	if m.cfg.UpdateVersion != "" && m.cfg.DryRun {
		out += "\n" + infoStyle.Render("↑ Nova versão disponível: "+m.cfg.UpdateVersion+" — execute sem --dry-run para atualizar.")
	}
	for _, w := range m.cfg.Sys.Warnings {
		out += "\n" + warnStyle.Width(max(20, m.w)).Render("! "+w)
	}
	return out + "\n"
}

func (m Model) View() string {
	if m.w == 0 {
		return ""
	}
	var v string
	switch m.state {
	case stateUser:
		v = m.viewUser()
	case stateAtualizacao:
		v = m.viewAtualizacao()
	case stateWallpaper:
		v = m.viewWallpaper()
	case stateExtensoes:
		v = m.viewExtensoes()
	case stateSelect:
		v = m.viewSelect()
	case stateConfirm:
		v = m.viewConfirm()
	case stateRunning:
		v = m.viewRunning()
	case stateDone:
		v = m.viewDone()
	}
	return lipgloss.NewStyle().Padding(1, 2, 0, 2).Render(v)
}
