// post-install configura uma workstation Ubuntu recém-instalada: mirrors,
// atualização, repositórios, Docker/VSCode/PHP/SQL Server, Flatpaks,
// Tailscale, Chrome, Bitwarden, temas, ambiente do usuário, Rust, Claude Code,
// OpenVPN e virt-manager — com uma TUI para escolher o que rodar.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/vinicgobbi/post-install/internal/engine"
	"github.com/vinicgobbi/post-install/internal/modules"
	"github.com/vinicgobbi/post-install/internal/plain"
	"github.com/vinicgobbi/post-install/internal/runlog"
	"github.com/vinicgobbi/post-install/internal/runner"
	"github.com/vinicgobbi/post-install/internal/sysinfo"
	"github.com/vinicgobbi/post-install/internal/tui"
	"github.com/vinicgobbi/post-install/internal/update"
)

var stdin = bufio.NewReader(os.Stdin)

// version é definido na compilação (-ldflags "-X main.version=...").
var version = "dev"

type options struct {
	user     string
	modules  string
	skip     string
	yes      bool
	noTUI    bool
	dryRun   bool
	failFast bool
	list     bool
	version  bool
	ovpnDir  string
	logDir   string
	wallDir  string
	wall     string
	gnomeDir string
	forceExt string
	noUpdate bool
}

func parseFlags() options {
	var o options
	flag.StringVar(&o.user, "user", "", "usuário do sistema que terá o ambiente configurado")
	flag.StringVar(&o.modules, "modules", "", "IDs dos módulos a selecionar, separados por vírgula (padrão: todos)")
	flag.StringVar(&o.skip, "skip", "", "IDs dos módulos a desmarcar, separados por vírgula")
	flag.BoolVar(&o.yes, "yes", false, "não pedir confirmação antes de executar (modo texto)")
	flag.BoolVar(&o.noTUI, "no-tui", false, "usar saída em texto simples em vez da interface interativa")
	flag.BoolVar(&o.dryRun, "dry-run", false, "só mostrar o que seria feito, sem alterar o sistema")
	flag.BoolVar(&o.failFast, "fail-fast", false, "parar tudo no primeiro módulo que falhar")
	flag.BoolVar(&o.list, "list", false, "listar os módulos disponíveis e sair")
	flag.BoolVar(&o.version, "version", false, "mostrar a versão e sair")
	flag.StringVar(&o.ovpnDir, "ovpn-dir", "", "pasta com os arquivos .ovpn a importar (padrão: OVPN/ ao lado do binário)")
	flag.StringVar(&o.wallDir, "wallpapers-dir", "", "pasta com as imagens de papel de parede (padrão: Wallpapers/ ao lado do binário)")
	flag.StringVar(&o.wall, "wallpaper", "", "papel de parede a aplicar: nome do arquivo em --wallpapers-dir, ou \"none\" para não alterar (padrão: perguntar)")
	flag.StringVar(&o.gnomeDir, "gnome-dir", "", "pasta com extensoes.txt e extensoes.dconf (padrão: GNOME/ ao lado do binário)")
	flag.StringVar(&o.forceExt, "force-extensions", "", "extensões do GNOME sem versão compatível a instalar mesmo assim (UUIDs separados por vírgula, ou \"none\"); sem ela, pergunta")
	flag.BoolVar(&o.noUpdate, "no-update-check", false, "não verificar se há uma versão mais nova do post-install")
	flag.StringVar(&o.logDir, "log-dir", "", "pasta onde os logs da execução são gravados (padrão: logs/ ao lado do binário)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Uso: sudo %s [opções]\n\nOpções:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	// Por padrão, OVPN/, Wallpapers/, GNOME/ e logs/ ficam ao lado do binário (estrutura do
	// pacote .tar.gz da release), para funcionar de qualquer diretório.
	base := exeDir()
	if o.ovpnDir == "" {
		o.ovpnDir = filepath.Join(base, "OVPN")
	}
	if o.logDir == "" {
		o.logDir = filepath.Join(base, "logs")
	}
	if o.wallDir == "" {
		o.wallDir = filepath.Join(base, "Wallpapers")
	}
	if o.gnomeDir == "" {
		o.gnomeDir = filepath.Join(base, "GNOME")
	}
	// Absolutos: os scripts como usuário rodam via "su -", que muda o
	// diretório atual para a home dele.
	for _, d := range []*string{&o.ovpnDir, &o.wallDir, &o.gnomeDir, &o.logDir} {
		if abs, err := filepath.Abs(*d); err == nil {
			*d = abs
		}
	}
	return o
}

// resolveWallpaper converte o valor de --wallpaper no caminho da imagem.
// Devolve ok=false quando a flag não foi usada (é preciso perguntar).
func resolveWallpaper(walls []string, name string) (path string, ok bool, err error) {
	switch name {
	case "":
		return "", false, nil
	case "none":
		return "", true, nil
	}
	for _, w := range walls {
		if filepath.Base(w) == name {
			return w, true, nil
		}
	}
	return "", false, fmt.Errorf("papel de parede %q não encontrado na pasta de papéis de parede", name)
}

// exePath devolve o caminho real do executável (sem links simbólicos).
func exePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// exeDir devolve a pasta do executável (ou o diretório atual, se não der
// para descobrir).
func exeDir() string {
	if exe, err := exePath(); err == nil {
		return filepath.Dir(exe)
	}
	return "."
}

// verificarAtualizacao consulta a última release (com prazo curto, para não
// atrasar o início sem rede). Falhas só viram aviso.
func verificarAtualizacao(out *plain.Printer) *update.Release {
	if version == "dev" {
		return nil
	}
	out.Info("Verificando se há versão nova do post-install...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rel, err := update.Check(ctx, version)
	if err != nil {
		out.Warn("Não foi possível verificar se há versão nova: %v", err)
		return nil
	}
	return rel
}

// comandoReexecucao monta o comando para executar de novo com as mesmas
// opções, depois de uma atualização.
func comandoReexecucao(exe string) string {
	partes := []string{"sudo", shellQuote(exe)}
	for _, a := range os.Args[1:] {
		partes = append(partes, shellQuote(a))
	}
	return strings.Join(partes, " ")
}

// shellQuote só põe aspas quando o argumento tem algo que o shell
// interpretaria.
func shellQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n'\"\\$`!*?[]{}()<>|&;#~") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func main() {
	os.Exit(run())
}

func run() int {
	o := parseFlags()
	all := modules.All()

	if o.version {
		fmt.Println("post-install", version)
		return 0
	}
	if o.list {
		for i, m := range all {
			if i == 0 || m.Grupo != all[i-1].Grupo {
				if i > 0 {
					fmt.Println()
				}
				fmt.Println(m.Grupo + ":")
			}
			deps := ""
			if len(m.Deps) > 0 {
				deps = " (requer: " + strings.Join(m.Deps, ", ") + ")"
			}
			fmt.Printf("  %-20s %s%s\n", m.ID, m.Title, deps)
		}
		return 0
	}

	out := &plain.Printer{W: os.Stdout, Color: term.IsTerminal(int(os.Stdout.Fd()))}
	fail := func(format string, a ...any) int {
		out.Handle(runlog.Entry{Level: runlog.Error, Text: fmt.Sprintf(format, a...)})
		return 1
	}

	if os.Geteuid() != 0 && !o.dryRun {
		return fail("Execute como root: sudo %s (ou use --dry-run para só simular).", os.Args[0])
	}

	sys, err := sysinfo.Detect()
	if err != nil {
		return fail("%v", err)
	}

	sel, err := initialSelection(all, o.modules, o.skip)
	if err != nil {
		return fail("%v", err)
	}

	if o.user != "" {
		if err := validateUser(o.user); err != nil {
			return fail("%v", err)
		}
	}

	log, err := runlog.New(o.logDir, "setup")
	if err != nil {
		return fail("%v", err)
	}
	defer log.Close()

	workDir, err := os.MkdirTemp("", "post-install-")
	if err != nil {
		return fail("%v", err)
	}
	defer os.RemoveAll(workDir)
	// Legível por todos: o apt instala os .deb baixados aqui como o usuário _apt.
	os.Chmod(workDir, 0o755)

	p := preparo{walls: modules.ListWallpapers(o.wallDir)}
	if !o.noUpdate {
		p.upd = verificarAtualizacao(out)
	}
	p.esc.Wallpaper, p.wallChosen, err = resolveWallpaper(p.walls, o.wall)
	if err != nil {
		return fail("%v", err)
	}
	if o.forceExt != "" {
		p.extChosen = true
		if o.forceExt != "none" {
			p.esc.ForcarExtensoes = splitList(o.forceExt)
		}
	}
	// Consulta antes da interface quais extensões do GNOME não têm versão
	// para o GNOME Shell da máquina, para perguntar se instala mesmo assim.
	if !p.extChosen && isSelected(all, sel, "extensoes_gnome") {
		out.Info("Verificando a compatibilidade das extensões do GNOME...")
		p.shell, p.incompat = modules.VerificarExtensoes(o.gnomeDir)
	}

	start := func(ctx context.Context, esc modules.Escolhas, mods []modules.Module, emit func(any)) []engine.Result {
		log.SetSink(func(e runlog.Entry) { emit(e) })
		log.Infof("Sistema detectado: %s", sys.Summary())
		log.Infof("Configuração será aplicada para: %s", esc.User)
		if log.CleanPath != "" {
			log.Infof("Log desta execução: %s", log.CleanPath)
			if !o.dryRun {
				if err := chownLogs(log, esc.User); err != nil {
					log.Warnf("Não foi possível passar os logs para %s: %v", esc.User, err)
				}
			}
		}
		c := &modules.Ctx{
			Runner:          &runner.Runner{Ctx: ctx, Log: log, User: esc.User, DryRun: o.dryRun, WorkDir: workDir},
			Sys:             sys,
			OVPNDir:         o.ovpnDir,
			Wallpaper:       esc.Wallpaper,
			GNOMEDir:        o.gnomeDir,
			ForcarExtensoes: esc.ForcarExtensoes,
		}
		return engine.Run(ctx, mods, c, engine.Options{FailFast: o.failFast}, emit)
	}

	interactive := !o.noTUI && term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
	if interactive {
		return runTUI(o, sys, all, sel, p, log, start)
	}
	return runPlain(o, sys, all, sel, p, out, start)
}

// preparo reúne o que é levantado antes da interface e as escolhas que já
// vieram por flag.
type preparo struct {
	esc        modules.Escolhas
	walls      []string
	wallChosen bool
	shell      string
	incompat   []modules.ExtInfo
	extChosen  bool
	upd        *update.Release // versão nova disponível (nil = nenhuma)
}

type startFunc func(ctx context.Context, esc modules.Escolhas, mods []modules.Module, emit func(any)) []engine.Result

func runTUI(o options, sys *sysinfo.Info, all []modules.Module, sel []bool, p preparo, log *runlog.Logger, start startFunc) int {
	cfg := tui.Config{
		Sys:              sys,
		Version:          version,
		DryRun:           o.dryRun,
		Modules:          all,
		Selected:         sel,
		User:             o.user,
		DetectedUser:     detectUser(),
		ValidateUser:     validateUser,
		Wallpapers:       p.walls,
		Wallpaper:        p.esc.Wallpaper,
		WallChosen:       p.wallChosen,
		ShellVersion:     p.shell,
		ExtIncompativeis: p.incompat,
		ForcarExtensoes:  p.esc.ForcarExtensoes,
		ExtChosen:        p.extChosen,
		LogPaths:         []string{log.CleanPath, log.RawPath},
		Start: func(ctx context.Context, esc modules.Escolhas, mods []modules.Module, emit func(any)) {
			start(ctx, esc, mods, emit)
		},
	}
	var rerun string
	if exe, err := exePath(); err == nil && p.upd != nil {
		rerun = comandoReexecucao(exe)
		cfg.UpdateVersion = p.upd.Tag
		cfg.RerunCommand = rerun
		cfg.ApplyUpdate = func() error { return update.Apply(context.Background(), p.upd, exe) }
	}
	final, err := tea.NewProgram(tui.New(cfg), tea.WithAltScreen()).Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro na interface:", err)
		return 1
	}
	if final.(tui.Model).Updated() {
		fmt.Printf("post-install atualizado para %s. Execute de novo para usar a versão nova:\n  %s\n", p.upd.Tag, rerun)
		return 0
	}
	res := final.(tui.Model).Result()
	if res == nil {
		fmt.Println("Nenhum módulo executado.")
		return 0
	}
	// A TUI roda em tela alternativa; reimprime o resumo para ficar no
	// histórico do terminal.
	fmt.Println(tui.Summary(res.Results, res.Elapsed, []string{log.CleanPath, log.RawPath}))
	return exitCode(res.Results)
}

func runPlain(o options, sys *sysinfo.Info, all []modules.Module, sel []bool, p preparo, out *plain.Printer, start startFunc) int {
	stdinTTY := term.IsTerminal(int(os.Stdin.Fd()))
	out.Info("Sistema detectado: %s", sys.Summary())
	for _, w := range sys.Warnings {
		out.Warn("%s", w)
	}

	if p.upd != nil {
		if code, sair := oferecerAtualizacao(o, p.upd, out, stdinTTY); sair {
			return code
		}
	}

	esc := p.esc
	esc.User = o.user
	if esc.User == "" {
		if !stdinTTY {
			out.Handle(runlog.Entry{Level: runlog.Error, Text: "Sem terminal interativo: informe o usuário com --user NOME."})
			return 1
		}
		esc.User = promptUser(out, detectUser())
	}

	if !p.wallChosen && len(p.walls) > 0 && isSelected(all, sel, "papel_parede") {
		if stdinTTY {
			esc.Wallpaper = promptWallpaper(out, p.walls)
		} else {
			out.Warn("Sem terminal interativo e sem --wallpaper: o papel de parede não será alterado.")
		}
	}

	if len(p.incompat) > 0 {
		out.Warn("Extensões sem versão compatível com o GNOME Shell %s: não serão instaladas automaticamente.", p.shell)
		for _, e := range p.incompat {
			if stdinTTY && confirm(fmt.Sprintf("Instalar %s mesmo assim (suporta até o GNOME %s; pode não funcionar corretamente)?", e.UUID, e.MaiorShell)) {
				esc.ForcarExtensoes = append(esc.ForcarExtensoes, e.UUID)
			} else if !stdinTTY {
				out.Warn("  - %s (suporta até o GNOME %s); use --force-extensions para instalar mesmo assim.", e.UUID, e.MaiorShell)
			}
		}
	}

	if o.modules == "" && !stdinTTY {
		out.Warn("Entrada não é um terminal interativo: executando todos os módulos (exceto os de --skip).")
	}
	for _, msg := range engine.ResolveDeps(all, sel) {
		out.Warn("%s", msg)
	}

	var mods []modules.Module
	for i, m := range all {
		if sel[i] {
			mods = append(mods, m)
		}
	}
	if len(mods) == 0 {
		out.Warn("Nenhum módulo selecionado. Nada a fazer.")
		return 0
	}

	out.Info("Módulos selecionados:")
	for _, m := range mods {
		fmt.Fprintln(out.W, "  - "+m.Title)
	}
	if stdinTTY && !o.yes && !confirm(fmt.Sprintf("Iniciar a configuração com os %d módulos acima?", len(mods))) {
		out.Warn("Operação cancelada pelo usuário.")
		return 0
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var elapsed engine.FinishedEvent
	results := start(ctx, esc, mods, func(ev any) {
		if f, ok := ev.(engine.FinishedEvent); ok {
			elapsed = f
		}
		out.Handle(ev)
	})
	fmt.Println()
	fmt.Println(tui.Summary(results, elapsed.Elapsed, nil))
	return exitCode(results)
}

// oferecerAtualizacao pergunta (modo texto) se atualiza para a versão nova.
// Devolve sair=true quando a execução deve parar (atualizou ou falhou).
func oferecerAtualizacao(o options, rel *update.Release, out *plain.Printer, stdinTTY bool) (code int, sair bool) {
	novidade := fmt.Sprintf("Nova versão disponível: %s (você está na %s).", rel.Tag, version)
	switch {
	case o.dryRun:
		out.Info("%s Execute sem --dry-run para atualizar.", novidade)
		return 0, false
	case !stdinTTY || o.yes:
		out.Warn("%s Execute de forma interativa para atualizar.", novidade)
		return 0, false
	case !confirmDefaultYes(novidade + " Atualizar agora (depois é preciso executar de novo)?"):
		return 0, false
	}
	exe, err := exePath()
	if err != nil {
		out.Handle(runlog.Entry{Level: runlog.Error, Text: fmt.Sprintf("Falha ao localizar o executável: %v", err)})
		return 1, true
	}
	out.Info("Baixando e conferindo %s...", rel.Tag)
	if err := update.Apply(context.Background(), rel, exe); err != nil {
		out.Handle(runlog.Entry{Level: runlog.Error, Text: fmt.Sprintf("Falha ao atualizar: %v", err)})
		return 1, true
	}
	out.Handle(runlog.Entry{Level: runlog.Success, Text: fmt.Sprintf("post-install atualizado para %s. Execute de novo para usar a versão nova:", rel.Tag)})
	fmt.Fprintln(out.W, "  "+comandoReexecucao(exe))
	return 0, true
}

func exitCode(results []engine.Result) int {
	code := 0
	for _, r := range results {
		switch r.Status {
		case engine.Canceled:
			return 130
		case engine.Failed:
			code = 1
		}
	}
	return code
}

// initialSelection marca todos os módulos (ou só os de --modules) e
// desmarca os de --skip.
func initialSelection(all []modules.Module, only, skip string) ([]bool, error) {
	ids := make([]string, len(all))
	for i, m := range all {
		ids[i] = m.ID
	}
	parse := func(s string) ([]string, error) {
		var out []string
		for _, id := range strings.Split(s, ",") {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if !slices.Contains(ids, id) {
				return nil, fmt.Errorf("módulo desconhecido: %q (veja --list)", id)
			}
			out = append(out, id)
		}
		return out, nil
	}
	onlyIDs, err := parse(only)
	if err != nil {
		return nil, err
	}
	skipIDs, err := parse(skip)
	if err != nil {
		return nil, err
	}
	sel := make([]bool, len(all))
	for i, id := range ids {
		sel[i] = (len(onlyIDs) == 0 || slices.Contains(onlyIDs, id)) && !slices.Contains(skipIDs, id)
	}
	return sel, nil
}

// chownLogs passa os logs desta execução para o usuário alvo.
func chownLogs(log *runlog.Logger, user string) error {
	ids := make([]int, 2)
	for i, flag := range []string{"-u", "-g"} {
		out, err := exec.Command("id", flag, "--", user).Output()
		if err != nil {
			return err
		}
		if ids[i], err = strconv.Atoi(strings.TrimSpace(string(out))); err != nil {
			return err
		}
	}
	return log.Chown(ids[0], ids[1])
}

// validateUser usa "id" (e não os/user) para resolver também contas de
// domínio (AD/LDAP via NSS), já que o binário é compilado sem cgo.
func validateUser(name string) error {
	if err := exec.Command("id", "-u", "--", name).Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("usuário '%s' não existe neste sistema", name)
		}
		return err
	}
	return nil
}

// detectUser descobre quem executou o programa via sudo/pkexec (ou o dono
// da sessão, via logname), para sugerir na pergunta do usuário. Ignora root.
func detectUser() string {
	cands := []string{os.Getenv("SUDO_USER")}
	if uid := os.Getenv("PKEXEC_UID"); uid != "" {
		if out, err := exec.Command("id", "-nu", uid).Output(); err == nil {
			cands = append(cands, strings.TrimSpace(string(out)))
		}
	}
	if out, err := exec.Command("logname").Output(); err == nil {
		cands = append(cands, strings.TrimSpace(string(out)))
	}
	for _, c := range cands {
		if c != "" && c != "root" && validateUser(c) == nil {
			return c
		}
	}
	return ""
}

func promptUser(out *plain.Printer, detected string) string {
	if detected != "" && confirmDefaultYes(fmt.Sprintf("Configurar o ambiente para o usuário '%s' (quem executou o sudo)?", detected)) {
		return detected
	}
	for {
		fmt.Fprint(out.W, "[?] Para qual usuário do sistema devo configurar o ambiente? ")
		line, err := stdin.ReadString('\n')
		if err != nil {
			os.Exit(1)
		}
		name := strings.TrimSpace(line)
		if name == "" {
			out.Warn("Digite um nome de usuário.")
			continue
		}
		if err := validateUser(name); err != nil {
			out.Warn("%v. Tente novamente.", err)
			continue
		}
		return name
	}
}

func splitList(s string) []string {
	var out []string
	for _, it := range strings.Split(s, ",") {
		if it = strings.TrimSpace(it); it != "" {
			out = append(out, it)
		}
	}
	return out
}

func isSelected(all []modules.Module, sel []bool, id string) bool {
	for i, m := range all {
		if m.ID == id {
			return sel[i]
		}
	}
	return false
}

// promptWallpaper pergunta qual papel de parede aplicar (modo texto).
func promptWallpaper(out *plain.Printer, walls []string) string {
	fmt.Fprintln(out.W, "[?] Qual papel de parede deseja aplicar?")
	for i, w := range walls {
		fmt.Fprintf(out.W, "  %d) %s\n", i+1, filepath.Base(w))
	}
	fmt.Fprintln(out.W, "  0) Não alterar o papel de parede")
	for {
		fmt.Fprint(out.W, "[?] Número: ")
		line, err := stdin.ReadString('\n')
		if err != nil {
			return ""
		}
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(line), "%d", &n); err == nil && n >= 0 && n <= len(walls) {
			if n == 0 {
				return ""
			}
			return walls[n-1]
		}
		out.Warn("Opção inválida.")
	}
}

// confirmDefaultYes é como confirm, mas Enter vazio conta como "sim".
func confirmDefaultYes(question string) bool {
	fmt.Printf("[?] %s [S/n]: ", question)
	line, _ := stdin.ReadString('\n')
	switch strings.TrimSpace(strings.ToLower(line)) {
	case "", "s", "y", "sim", "yes":
		return true
	}
	return false
}

func confirm(question string) bool {
	fmt.Printf("[?] %s [s/N]: ", question)
	line, _ := stdin.ReadString('\n')
	switch strings.TrimSpace(strings.ToLower(line)) {
	case "s", "y", "sim", "yes":
		return true
	}
	return false
}
