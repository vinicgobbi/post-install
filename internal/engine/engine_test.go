package engine

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/vinicgobbi/post-install/internal/modules"
	"github.com/vinicgobbi/post-install/internal/runlog"
	"github.com/vinicgobbi/post-install/internal/runner"
)

func fakeMods(runs map[string]error) []modules.Module {
	mk := func(id string, deps ...string) modules.Module {
		return modules.Module{ID: id, Title: id, Deps: deps, Run: func(*modules.Ctx) error { return runs[id] }}
	}
	return []modules.Module{mk("a"), mk("b", "a"), mk("c", "b"), mk("d")}
}

func TestSelectWithDepsChain(t *testing.T) {
	mods := fakeMods(nil)
	sel := make([]bool, len(mods))
	added := SelectWithDeps(mods, sel, 2)
	if !slices.Equal(sel, []bool{true, true, true, false}) {
		t.Fatalf("sel = %v", sel)
	}
	if !slices.Equal(added, []string{"b", "a"}) {
		t.Fatalf("added = %v", added)
	}
}

func TestDeselectWithDependents(t *testing.T) {
	mods := fakeMods(nil)
	sel := []bool{true, true, true, true}
	removed := DeselectWithDependents(mods, sel, 0)
	if !slices.Equal(sel, []bool{false, false, false, true}) {
		t.Fatalf("sel = %v", sel)
	}
	if !slices.Equal(removed, []string{"b", "c"}) {
		t.Fatalf("removed = %v", removed)
	}
}

func TestRealModulesDepsExist(t *testing.T) {
	all := modules.All()
	ids := index(all)
	for i, m := range all {
		for _, d := range m.Deps {
			j, ok := ids[d]
			if !ok {
				t.Errorf("%s depende de %q, que não existe", m.ID, d)
			} else if j >= i {
				t.Errorf("%s depende de %s, que roda depois dele", m.ID, d)
			}
		}
	}
}

func TestRunSkipsDependentsOfFailure(t *testing.T) {
	mods := fakeMods(map[string]error{"a": errors.New("boom")})
	log, _ := runlog.New("", "test")
	c := &modules.Ctx{Runner: &runner.Runner{Ctx: context.Background(), Log: log}}

	res := Run(context.Background(), mods, c, Options{}, func(any) {})
	want := []Status{Failed, Skipped, Skipped, Done}
	for i, r := range res {
		if r.Status != want[i] {
			t.Errorf("%s: status %v, esperado %v", r.Module.ID, r.Status, want[i])
		}
	}

	res = Run(context.Background(), mods, c, Options{FailFast: true}, func(any) {})
	if res[3].Status != Canceled {
		t.Errorf("fail-fast: d deveria ser cancelado, veio %v", res[3].Status)
	}
}
