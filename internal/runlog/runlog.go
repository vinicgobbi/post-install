// Package runlog grava o log de cada execução e repassa as mensagens para a
// interface (TUI ou modo texto).
//
// Cada execução gera dois arquivos no diretório de logs:
//
//	<prefixo>_<timestamp>.log      só as mensagens info/aviso/erro/sucesso/passo,
//	                               com hora e nível, sem código de cor.
//	<prefixo>_<timestamp>.raw.log  transcrição bruta e completa, incluindo a
//	                               saída crua de apt/dnf/curl/flatpak etc.
package runlog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Level int

const (
	Info Level = iota
	Success
	Warn
	Error
	Step
	// Output é uma linha de saída crua de um comando externo.
	Output
)

func (l Level) String() string {
	switch l {
	case Info:
		return "INFO"
	case Success:
		return "OK"
	case Warn:
		return "AVISO"
	case Error:
		return "ERRO"
	case Step:
		return "PASSO"
	default:
		return "SAIDA"
	}
}

type Entry struct {
	Level Level
	Text  string
	Time  time.Time
}

type Logger struct {
	mu       sync.Mutex
	clean    *os.File
	raw      *os.File
	sink     func(Entry)
	warnings atomic.Int64

	CleanPath string
	RawPath   string
}

// New cria os arquivos de log em dir. Com dir vazio, nada é gravado em disco
// (as mensagens só vão para o sink).
func New(dir, prefixo string) (*Logger, error) {
	l := &Logger{}
	if dir == "" {
		return l, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("criando diretório de logs: %w", err)
	}
	ts := time.Now().Format("2006-01-02_150405")
	l.CleanPath = filepath.Join(dir, fmt.Sprintf("%s_%s.log", prefixo, ts))
	l.RawPath = filepath.Join(dir, fmt.Sprintf("%s_%s.raw.log", prefixo, ts))

	var err error
	if l.clean, err = os.Create(l.CleanPath); err != nil {
		return nil, err
	}
	if l.raw, err = os.Create(l.RawPath); err != nil {
		l.clean.Close()
		return nil, err
	}
	return l, nil
}

// SetSink define quem recebe cada mensagem (a TUI ou o modo texto).
func (l *Logger) SetSink(fn func(Entry)) {
	l.mu.Lock()
	l.sink = fn
	l.mu.Unlock()
}

func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.clean != nil {
		l.clean.Close()
	}
	if l.raw != nil {
		l.raw.Close()
	}
}

// Warnings devolve quantos avisos foram emitidos até agora (o engine usa a
// diferença antes/depois de cada módulo para marcar "concluído com avisos").
func (l *Logger) Warnings() int64 { return l.warnings.Load() }

func (l *Logger) Infof(format string, a ...any)    { l.emit(Info, fmt.Sprintf(format, a...)) }
func (l *Logger) Successf(format string, a ...any) { l.emit(Success, fmt.Sprintf(format, a...)) }
func (l *Logger) Warnf(format string, a ...any)    { l.emit(Warn, fmt.Sprintf(format, a...)) }
func (l *Logger) Errorf(format string, a ...any)   { l.emit(Error, fmt.Sprintf(format, a...)) }
func (l *Logger) Stepf(format string, a ...any)    { l.emit(Step, fmt.Sprintf(format, a...)) }

func (l *Logger) emit(level Level, text string) {
	if level == Warn {
		l.warnings.Add(1)
	}
	now := time.Now()
	l.mu.Lock()
	if l.clean != nil {
		fmt.Fprintf(l.clean, "%s [%s] %s\n", now.Format("2006-01-02 15:04:05"), level, text)
	}
	if l.raw != nil {
		fmt.Fprintf(l.raw, "[%s] %s\n", level, text)
	}
	sink := l.sink
	l.mu.Unlock()
	if sink != nil {
		sink(Entry{Level: level, Text: text, Time: now})
	}
}

// RawWrite grava texto só no .raw.log (saída de consultas silenciosas).
func (l *Logger) RawWrite(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.raw != nil {
		l.raw.WriteString(s)
	}
}

// OutputWriter devolve um io.Writer para a saída de um comando externo: os
// bytes vão crus para o .raw.log e cada linha (quebrada em \n ou \r, para
// barras de progresso) é repassada ao sink como Output.
func (l *Logger) OutputWriter() io.WriteCloser {
	return &lineWriter{l: l}
}

type lineWriter struct {
	l   *Logger
	buf []byte
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.l.mu.Lock()
	if w.l.raw != nil {
		w.l.raw.Write(p)
	}
	w.l.mu.Unlock()

	w.buf = append(w.buf, p...)
	for {
		i := strings.IndexAny(string(w.buf), "\r\n")
		if i < 0 {
			break
		}
		w.flushLine(string(w.buf[:i]))
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}

func (w *lineWriter) Close() error {
	if len(w.buf) > 0 {
		w.flushLine(string(w.buf))
		w.buf = nil
	}
	return nil
}

func (w *lineWriter) flushLine(line string) {
	line = strings.TrimRight(line, " \t")
	if line == "" {
		return
	}
	w.l.mu.Lock()
	sink := w.l.sink
	w.l.mu.Unlock()
	if sink != nil {
		sink(Entry{Level: Output, Text: line, Time: time.Now()})
	}
}
