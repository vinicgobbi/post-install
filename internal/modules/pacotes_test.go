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

func TestGcmTarGz(t *testing.T) {
	for nome, want := range map[string]bool{
		"gcm-linux-x64-3.0.2.tar.gz":         true,
		"gcm-linux-x64-3.0.2-symbols.tar.gz": false,
		"gcm-linux-arm64-3.0.2.tar.gz":       false,
		"gcm-linux-x64-3.0.2.deb":            false,
	} {
		if got := gcmTarGz(nome); got != want {
			t.Errorf("gcmTarGz(%q) = %v, esperado %v", nome, got, want)
		}
	}
}

// O formato com "|" e "sistema" resolve a mesma lista para cada distribuição.
func TestListaExtensoesPorDistro(t *testing.T) {
	lista := `# comentário
comum@exemplo.test
ubuntu-dock@ubuntu.com | dash-to-dock@micxgx.gmail.com
a@ubuntu.com|b@exemplo.test|c@exemplo.test
snapd-prompting@canonical.com sistema
 | 
git@exemplo.test https://git.exemplo.test/x.git abc123
-ding@rastersoft.com
`
	maquina := func(sistema ...string) *runner.Runner {
		fs := fstest.MapFS{"lista.txt": {Data: []byte(lista)}}
		for _, uuid := range sistema {
			fs["usr/share/gnome-shell/extensions/"+uuid+"/metadata.json"] = &fstest.MapFile{Data: []byte("{}")}
		}
		return &runner.Runner{Host: &hostFalso{arquivos: fs, semResposta: map[string]bool{}}}
	}

	// Ubuntu: as do sistema entram, e nenhuma equivalente é baixada.
	ativar, desativar, git, err := lerListaExtensoes(maquina("ubuntu-dock@ubuntu.com", "a@ubuntu.com", "snapd-prompting@canonical.com"), "/lista.txt")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"comum@exemplo.test", "ubuntu-dock@ubuntu.com", "a@ubuntu.com", "snapd-prompting@canonical.com", "git@exemplo.test"}; !slices.Equal(ativar, want) {
		t.Errorf("Ubuntu: ativar = %v, esperado %v", ativar, want)
	}
	if !slices.Equal(desativar, []string{"ding@rastersoft.com"}) || git["git@exemplo.test"].Ref != "abc123" || len(git) != 1 {
		t.Errorf("Ubuntu: desativar = %v, git = %v", desativar, git)
	}

	// Fedora: sem nada do sistema, entra a última de cada linha com "|", e
	// a marcada "sistema" fica de fora.
	ativar, _, _, err = lerListaExtensoes(maquina(), "/lista.txt")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"comum@exemplo.test", "dash-to-dock@micxgx.gmail.com", "c@exemplo.test", "git@exemplo.test"}; !slices.Equal(ativar, want) {
		t.Errorf("Fedora: ativar = %v, esperado %v", ativar, want)
	}

	// A primeira que vier com o sistema ganha, mesmo que não seja a primeira da linha.
	ativar, _, _, _ = lerListaExtensoes(maquina("b@exemplo.test"), "/lista.txt")
	if !slices.Contains(ativar, "b@exemplo.test") || slices.Contains(ativar, "c@exemplo.test") {
		t.Errorf("alternativa do sistema não escolhida: %v", ativar)
	}
}
