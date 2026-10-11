// post-install configura uma workstation Linux recém-instalada: mirrors,
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
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/vinicgobbi/post-install/internal/engine"
	"github.com/vinicgobbi/post-install/internal/modules"
	"github.com/vinicgobbi/post-install/internal/plain"
	"github.com/vinicgobbi/post-install/internal/runlog"
	"github.com/vinicgobbi/post-install/internal/runner"
	"github.com/vinicgobbi/post-install/internal/sysinfo"
	"github.com/vinicgobbi/post-install/internal/tui"
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
	flag.StringVar(&o.logDir, "log-dir", "", "pasta onde os logs da execução são gravados (padrão: logs/ ao lado do binário)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Uso: sudo %s [opções]\n\nOpções:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	// Por padrão, OVPN/, Wallpapers/ e logs/ ficam ao lado do binário (estrutura do
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
	// Absolutos: os scripts como usuário rodam via "su -", que muda o
	// diretório atual para a home dele.
	for _, d := range []*string{&o.ovpnDir, &o.wallDir, &o.logDir} {
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

// exeDir devolve a pasta do executável (ou o diretório atual, se não der
// para descobrir).
func exeDir() string {
	exe, err := os.Executable()
	if err == nil {
		if exe, err = filepath.EvalSymlinks(exe); err == nil {
			return filepath.Dir(exe)
		}
	}
	return "."
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
		for _, m := range all {
			deps := ""
			if len(m.Deps) > 0 {
				deps = " (requer: " + strings.Join(m.Deps, ", ") + ")"
			}
			fmt.Printf("%-20s %s%s\n", m.ID, m.Title, deps)
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

	walls := modules.ListWallpapers(o.wallDir)
	wallpaper, wallChosen, err := resolveWallpaper(walls, o.wall)
	if err != nil {
		return fail("%v", err)
	}

	start := func(ctx context.Context, user, wallpaper string, mods []modules.Module, emit func(any)) []engine.Result {
		log.SetSink(func(e runlog.Entry) { emit(e) })
		log.Infof("Sistema detectado: %s", sys.Summary())
		log.Infof("Configuração será aplicada para: %s", user)
		if log.CleanPath != "" {
			log.Infof("Log desta execução: %s", log.CleanPath)
		}
		c := &modules.Ctx{
			Runner:    &runner.Runner{Ctx: ctx, Log: log, User: user, DryRun: o.dryRun, WorkDir: workDir},
			Sys:       sys,
			OVPNDir:   o.ovpnDir,
			Wallpaper: wallpaper,
		}
		return engine.Run(ctx, mods, c, engine.Options{FailFast: o.failFast}, emit)
	}

	interactive := !o.noTUI && term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
	if interactive {
		return runTUI(o, sys, all, sel, walls, wallpaper, wallChosen, log, start)
	}
	return runPlain(o, sys, all, sel, walls, wallpaper, wallChosen, out, start)
}

type startFunc func(ctx context.Context, user, wallpaper string, mods []modules.Module, emit func(any)) []engine.Result

func runTUI(o options, sys *sysinfo.Info, all []modules.Module, sel []bool, walls []string, wallpaper string, wallChosen bool, log *runlog.Logger, start startFunc) int {
	m := tui.New(tui.Config{
		Sys:          sys,
		Version:      version,
		DryRun:       o.dryRun,
		Modules:      all,
		Selected:     sel,
		User:         o.user,
		ValidateUser: validateUser,
		Wallpapers:   walls,
		Wallpaper:    wallpaper,
		WallChosen:   wallChosen,
		LogPaths:     []string{log.CleanPath, log.RawPath},
		Start: func(ctx context.Context, user, wallpaper string, mods []modules.Module, emit func(any)) {
			start(ctx, user, wallpaper, mods, emit)
		},
	})
	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro na interface:", err)
		return 1
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

func runPlain(o options, sys *sysinfo.Info, all []modules.Module, sel []bool, walls []string, wallpaper string, wallChosen bool, out *plain.Printer, start startFunc) int {
	stdinTTY := term.IsTerminal(int(os.Stdin.Fd()))
	out.Info("Sistema detectado: %s", sys.Summary())
	for _, w := range sys.Warnings {
		out.Warn("%s", w)
	}

	user := o.user
	if user == "" {
		if !stdinTTY {
			out.Handle(runlog.Entry{Level: runlog.Error, Text: "Sem terminal interativo: informe o usuário com --user NOME."})
			return 1
		}
		user = promptUser(out, detectUser())
	}

	if !wallChosen && len(walls) > 0 && isSelected(all, sel, "papel_parede") {
		if stdinTTY {
			wallpaper = promptWallpaper(out, walls)
		} else {
			out.Warn("Sem terminal interativo e sem --wallpaper: o papel de parede não será alterado.")
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
	results := start(ctx, user, wallpaper, mods, func(ev any) {
		if f, ok := ev.(engine.FinishedEvent); ok {
			elapsed = f
		}
		out.Handle(ev)
	})
	fmt.Println()
	fmt.Println(tui.Summary(results, elapsed.Elapsed, nil))
	return exitCode(results)
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

// validateUser usa "id" (e não os/user) para resolver também contas de
// domínio (AD/LDAP via NSS), já que o binário é compilado sem cgo.
func validateUser(name string) error {
	if err := exec.Command("id", "-u", name).Run(); err != nil {
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
