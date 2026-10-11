// Package engine resolve dependências entre módulos e os executa em ordem,
// emitindo eventos para a interface.
package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vinicgobbi/post-install/internal/modules"
	"github.com/vinicgobbi/post-install/internal/runlog"
)

type Status int

const (
	Pending Status = iota
	Running
	Done
	DoneWithWarnings
	Failed
	Skipped
	Canceled
)

type Result struct {
	Module   modules.Module
	Status   Status
	Err      error
	Duration time.Duration
	Warnings int64
}

// Eventos enviados para a interface durante Run.
type (
	LogEvent         runlog.Entry
	ModuleStartEvent struct{ Index int }
	ModuleDoneEvent  struct {
		Index  int
		Result Result
	}
	FinishedEvent struct {
		Results []Result
		Elapsed time.Duration
	}
)

// index devolve a posição de cada ID em mods.
func index(mods []modules.Module) map[string]int {
	idx := make(map[string]int, len(mods))
	for i, m := range mods {
		idx[m.ID] = i
	}
	return idx
}

// SelectWithDeps marca o módulo i e, recursivamente, as dependências dele.
// Devolve os títulos dos módulos que foram marcados automaticamente.
func SelectWithDeps(mods []modules.Module, sel []bool, i int) []string {
	idx := index(mods)
	var added []string
	var visit func(int)
	visit = func(i int) {
		for _, dep := range mods[i].Deps {
			j, ok := idx[dep]
			if !ok || sel[j] {
				continue
			}
			sel[j] = true
			added = append(added, mods[j].Title)
			visit(j)
		}
	}
	sel[i] = true
	visit(i)
	return added
}

// DeselectWithDependents desmarca o módulo i e, recursivamente, os módulos
// marcados que dependem dele. Devolve os títulos desmarcados em cascata.
func DeselectWithDependents(mods []modules.Module, sel []bool, i int) []string {
	var removed []string
	var visit func(int)
	visit = func(i int) {
		sel[i] = false
		for j, m := range mods {
			if !sel[j] {
				continue
			}
			for _, dep := range m.Deps {
				if dep == mods[i].ID {
					removed = append(removed, m.Title)
					visit(j)
					break
				}
			}
		}
	}
	visit(i)
	return removed
}

// ResolveDeps garante que toda dependência de um módulo selecionado também
// esteja selecionada (usado quando a seleção vem de flags, sem a TUI).
func ResolveDeps(mods []modules.Module, sel []bool) []string {
	var msgs []string
	for i := range mods {
		if !sel[i] {
			continue
		}
		for _, t := range SelectWithDeps(mods, sel, i) {
			msgs = append(msgs, fmt.Sprintf("'%s' depende de '%s': selecionando automaticamente.", mods[i].Title, t))
		}
	}
	return msgs
}

type Options struct {
	// FailFast interrompe tudo no primeiro módulo com erro. Sem ela, os
	// módulos seguintes continuam rodando — exceto os que dependem (direta
	// ou indiretamente) do que falhou, que são pulados.
	FailFast bool
}

// Run executa os módulos selecionados, em ordem. emit recebe os eventos
// (ModuleStartEvent, ModuleDoneEvent, FinishedEvent); as mensagens de log
// chegam pelo sink do Logger.
func Run(ctx context.Context, mods []modules.Module, c *modules.Ctx, opts Options, emit func(any)) []Result {
	start := time.Now()
	results := make([]Result, len(mods))
	broken := map[string]bool{} // IDs que falharam ou foram pulados
	for i, m := range mods {
		results[i] = Result{Module: m, Status: Pending}
	}

	stop := false
	for i, m := range mods {
		if stop || ctx.Err() != nil {
			results[i].Status = Canceled
			emit(ModuleDoneEvent{Index: i, Result: results[i]})
			continue
		}
		if dep := firstBroken(m, broken); dep != "" {
			results[i].Status = Skipped
			results[i].Err = fmt.Errorf("dependência '%s' não concluída", dep)
			broken[m.ID] = true
			c.Log.Warnf("Pulando '%s': %v.", m.Title, results[i].Err)
			emit(ModuleDoneEvent{Index: i, Result: results[i]})
			continue
		}

		emit(ModuleStartEvent{Index: i})
		results[i].Status = Running
		c.Log.Stepf("[%d/%d] %s", i+1, len(mods), m.Title)

		warnBefore := c.Log.Warnings()
		t0 := time.Now()
		err := runSafe(m, c)
		results[i].Duration = time.Since(t0)
		results[i].Warnings = c.Log.Warnings() - warnBefore

		switch {
		case err != nil && (ctx.Err() != nil || errors.Is(err, context.Canceled)):
			results[i].Status = Canceled
			results[i].Err = err
			stop = true
		case err != nil:
			results[i].Status = Failed
			results[i].Err = err
			broken[m.ID] = true
			c.Log.Errorf("'%s' falhou: %v", m.Title, err)
			stop = opts.FailFast
		case results[i].Warnings > 0:
			results[i].Status = DoneWithWarnings
		default:
			results[i].Status = Done
		}
		emit(ModuleDoneEvent{Index: i, Result: results[i]})
	}

	emit(FinishedEvent{Results: results, Elapsed: time.Since(start)})
	return results
}

func firstBroken(m modules.Module, broken map[string]bool) string {
	for _, d := range m.Deps {
		if broken[d] {
			return d
		}
	}
	return ""
}

// runSafe transforma um panic dentro de um módulo em erro, para um bug num
// módulo não derrubar a interface inteira.
func runSafe(m modules.Module, c *modules.Ctx) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return m.Run(c)
}
