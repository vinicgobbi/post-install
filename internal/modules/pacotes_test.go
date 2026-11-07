package modules

import (
	"context"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/vinicgobbi/post-install/internal/runlog"
	"github.com/vinicgobbi/post-install/internal/runner"
	"github.com/vinicgobbi/post-install/internal/sysinfo"
)

// ctxDryRun monta um Ctx em dry-run numa máquina falsa da família dada e
// devolve também as linhas de log.
func ctxDryRun(t *testing.T, f sysinfo.Familia, h *hostFalso) (*Ctx, *[]string) {
	t.Helper()
	log, err := runlog.New("", "")
	if err != nil {
		t.Fatal(err)
	}
	var linhas []string
	log.SetSink(func(e runlog.Entry) { linhas = append(linhas, e.Text) })
	if h == nil {
		h = &hostFalso{arquivos: fstest.MapFS{}, binarios: map[string]bool{}, comandos: map[string]string{}, http: map[string]string{}, semResposta: map[string]bool{}}
	}
	return &Ctx{
		Runner: &runner.Runner{Ctx: context.Background(), Log: log, User: "ana", DryRun: true, WorkDir: "/tmp/teste", Host: h},
		Sys:    &sysinfo.Info{Familia: f},
	}, &linhas
}

func TestPacotesPorFamilia(t *testing.T) {
	for f, want := range map[sysinfo.Familia]string{
		sysinfo.Ubuntu: "[dry-run] apt install -y git curl",
		sysinfo.Fedora: "[dry-run] dnf install -y git curl",
	} {
		c, linhas := ctxDryRun(t, f, nil)
		if err := c.install("git", "curl"); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(*linhas, []string{want}) {
			t.Errorf("%s: %q, esperado %q", f, *linhas, want)
		}
	}

	h := &hostFalso{comandos: map[string]string{"rpm -q nautilus-python": "nautilus-python-4.0\n", "dpkg -s python3-nautilus": "ok\n"}, semResposta: map[string]bool{}}
	fed, _ := ctxDryRun(t, sysinfo.Fedora, h)
	ubu, _ := ctxDryRun(t, sysinfo.Ubuntu, h)
	if !fed.pacoteInstalado("nautilus-python") || fed.pacoteInstalado("python3-nautilus") {
		t.Error("Fedora deveria consultar o rpm")
	}
	if !ubu.pacoteInstalado("python3-nautilus") || ubu.pacoteInstalado("nautilus-python") {
		t.Error("Ubuntu deveria consultar o dpkg")
	}
}

func TestRepositorioRPM(t *testing.T) {
	c, linhas := ctxDryRun(t, sysinfo.Fedora, nil)
	chave, err := instalarChaveRPM(c, "https://exemplo.test/chave.asc", "exemplo.asc", "FINGERPRINT")
	if err != nil {
		t.Fatal(err)
	}
	if chave != "/etc/pki/rpm-gpg/exemplo.asc" {
		t.Errorf("chave em %s", chave)
	}
	if err := repoRPM(c, "exemplo", "Exemplo", "https://exemplo.test/rpm", chave); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"[dry-run] baixar https://exemplo.test/chave.asc -> /tmp/teste/exemplo.asc",
		"[dry-run] install -D -m 0644 /tmp/teste/exemplo.asc /etc/pki/rpm-gpg/exemplo.asc",
		"[dry-run] escrever /etc/yum.repos.d/exemplo.repo (120 bytes)",
	}
	if !slices.Equal(*linhas, want) {
		t.Errorf("passos:\n%s\nesperado:\n%s", strings.Join(*linhas, "\n"), strings.Join(want, "\n"))
	}
	repo := conteudoRepoRPM("exemplo", "Exemplo", "https://exemplo.test/rpm", chave)
	if repo != `[exemplo]
name=Exemplo
baseurl=https://exemplo.test/rpm
enabled=1
gpgcheck=1
gpgkey=file:///etc/pki/rpm-gpg/exemplo.asc
` {
		t.Errorf("repo:\n%s", repo)
	}
}

// Sem LibreOffice instalado, o Fedora não roda o "dnf remove" (que, com um
// padrão sem nada instalado, não tem sucesso garantido).
func TestRemoverLibreofficeFedoraSemNada(t *testing.T) {
	c, linhas := ctxDryRun(t, sysinfo.Fedora, nil)
	m, ok := removerLibreoffice.Para(sysinfo.Fedora)
	if !ok {
		t.Fatal("remover_libreoffice deveria existir no Fedora")
	}
	if err := m.Run(c); err != nil {
		t.Fatal(err)
	}
	for _, l := range *linhas {
		if strings.Contains(l, "dnf remove") {
			t.Errorf("não deveria remover nada: %q", *linhas)
		}
	}
}
