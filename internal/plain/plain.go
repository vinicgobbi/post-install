// Package plain é a saída em texto simples, usada quando não há terminal
// interativo (pipe, CI) ou com --no-tui.
package plain

import (
	"fmt"
	"io"
	"sync"

	"github.com/vinicgobbi/post-install/internal/runlog"
)

const (
	verde    = "\033[0;32m"
	amarelo  = "\033[1;33m"
	vermelho = "\033[0;31m"
	azul     = "\033[0;34m"
	cinza    = "\033[0;90m"
	negrito  = "\033[1m"
	nc       = "\033[0m"
)

// Printer imprime as mensagens do log e os eventos do engine.
type Printer struct {
	mu    sync.Mutex
	W     io.Writer
	Color bool
}

func (p *Printer) c(code, s string) string {
	if !p.Color {
		return s
	}
	return code + s + nc
}

// Handle imprime entradas de log; os demais eventos do engine são ignorados
// (o progresso já aparece nas linhas "[n/total]" do log).
func (p *Printer) Handle(ev any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch ev := ev.(type) {
	case runlog.Entry:
		switch ev.Level {
		case runlog.Info:
			fmt.Fprintln(p.W, p.c(amarelo, "[*] "+ev.Text))
		case runlog.Success:
			fmt.Fprintln(p.W, p.c(verde, "[+] "+ev.Text))
		case runlog.Warn:
			fmt.Fprintln(p.W, p.c(azul, "[!] "+ev.Text))
		case runlog.Error:
			fmt.Fprintln(p.W, p.c(vermelho, "[-] "+ev.Text))
		case runlog.Step:
			fmt.Fprintln(p.W, p.c(negrito+azul, ev.Text))
		default:
			fmt.Fprintln(p.W, p.c(cinza, ev.Text))
		}
	}
}

func (p *Printer) Info(format string, a ...any) {
	p.Handle(runlog.Entry{Level: runlog.Info, Text: fmt.Sprintf(format, a...)})
}

func (p *Printer) Warn(format string, a ...any) {
	p.Handle(runlog.Entry{Level: runlog.Warn, Text: fmt.Sprintf(format, a...)})
}
