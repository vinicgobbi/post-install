package tui

import "testing"

func TestHumanSize(t *testing.T) {
	for n, want := range map[int64]string{48248: "48 KB", 999_400: "999 KB", 1_715_251: "1,7 MB", 3_493_524: "3,5 MB"} {
		if got := humanSize(n); got != want {
			t.Errorf("humanSize(%d) = %q, esperado %q", n, got, want)
		}
	}
}
